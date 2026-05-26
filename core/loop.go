package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// UserInput represents input from the user.
type UserInput struct {
	Text string
}

// RunResult is the final result of a Run.
type RunResult struct {
	TurnCount int
	FinalMsg  string
	Usage     Usage
}

// TurnEndReason classifies why a turn ended.
type TurnEndReason string

const (
	ReasonComplete  TurnEndReason = "complete"
	ReasonToolCalls TurnEndReason = "tool_calls"
	ReasonError     TurnEndReason = "error"
	ReasonCancelled TurnEndReason = "cancelled"
)

// Loop drives one agent turn cycle.
type Loop interface {
	Prompt(ctx context.Context, sess *Session, input UserInput) (*Run, error)
	Continue(ctx context.Context, sess *Session) (*Run, error)
}

type defaultLoop struct{}

// NewLoop creates a new Loop.
func NewLoop() Loop {
	return &defaultLoop{}
}

// Prompt starts a new turn.
func (l *defaultLoop) Prompt(ctx context.Context, sess *Session, input UserInput) (*Run, error) {
	// 1. Add user message to transcript
	userMsg := Message{
		Role:      RoleUser,
		Content:   input.Text,
		MessageID: generateMsgID(),
	}
	sess.Transcript.Append(userMsg)

	// Consume steer queue — inject as system message before this turn
	if _, steerText := sess.DrainSteers(); steerText != "" {
		sess.Transcript.Append(Message{
			Role:      RoleSystem,
			Content:   "[User direction]\n" + steerText,
			MessageID: generateMsgID(),
		})
	}

	// 2. Build system prompt
	systemPrompt := ""
	if sess.SystemPrompt != nil {
		sp, err := sess.SystemPrompt(sess)
		if err != nil {
			return nil, fmt.Errorf("system prompt: %w", err)
		}
		systemPrompt = sp
	}

	// 3. Build tool specs from active tools
	var toolSpecs []ToolSpec
	for _, t := range sess.Tools.Active() {
		// Filter by Session.ActiveTools if set
		if len(sess.ActiveTools) > 0 && !contains(sess.ActiveTools, t.Name) {
			continue
		}
		schemaJSON, err := t.Schema.Marshal()
		if err != nil {
			return nil, fmt.Errorf("tool %s schema marshal: %w", t.Name, err)
		}
		toolSpecs = append(toolSpecs, ToolSpec{
			Name:        t.Name,
			Description: t.Description,
			Schema:      schemaJSON,
		})
	}

	req := StreamRequest{
		Model:        sess.DefaultModel,
		Messages:     sess.Transcript.Messages(),
		Tools:        toolSpecs,
		SystemPrompt: systemPrompt,
	}

	// 4. Call provider
	p, err := sess.ResolveProvider()
	if err != nil {
		return nil, err
	}
	provEvents, err := p.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider stream: %w", err)
	}

	// 5. Build Run
	run := &Run{
		loop:   l,
		sess:   sess,
		events: make(chan AgentEvent, 64),
		done:   make(chan struct{}),
	}

	// 6. Start event processing goroutine
	go run.processProviderEvents(ctx, provEvents, req)

	return run, nil
}

// Continue resumes a session — skeleton for demo.
// Continue resumes a session after tool calls complete, sending results back to the LLM.
// It does NOT add a user message — the transcript already contains assistant tool_calls + tool results.
func (l *defaultLoop) Continue(ctx context.Context, sess *Session) (*Run, error) {
	// 1. Build system prompt
	systemPrompt := ""
	if sess.SystemPrompt != nil {
		sp, err := sess.SystemPrompt(sess)
		if err != nil {
			return nil, fmt.Errorf("system prompt: %w", err)
		}
		systemPrompt = sp
	}

	// 2. Build tool specs from active tools
	var toolSpecs []ToolSpec
	for _, t := range sess.Tools.Active() {
		schemaJSON, err := t.Schema.Marshal()
		if err != nil {
			return nil, fmt.Errorf("tool %s schema marshal: %w", t.Name, err)
		}
		toolSpecs = append(toolSpecs, ToolSpec{
			Name:        t.Name,
			Description: t.Description,
			Schema:      schemaJSON,
		})
	}

	// 3. Build stream request from current transcript (already has tool results)
	// TODO: hardcoded model — demo only uses one provider
	req := StreamRequest{
		Model:        sess.DefaultModel,
		Messages:     sess.Transcript.Messages(),
		Tools:        toolSpecs,
		SystemPrompt: systemPrompt,
	}

	// 4. Call provider
	p, err := sess.ResolveProvider()
	if err != nil {
		return nil, err
	}
	provEvents, err := p.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("provider stream: %w", err)
	}

	// 5. Build Run
	run := &Run{
		loop:   l,
		sess:   sess,
		events: make(chan AgentEvent, 64),
		done:   make(chan struct{}),
	}

	go run.processProviderEvents(ctx, provEvents, req)
	return run, nil
}

// Run represents an in-flight turn.
type Run struct {
	loop     Loop
	sess     *Session
	events   chan AgentEvent
	done     chan struct{}
	result   RunResult
	resultMu sync.Mutex
	doneOnce sync.Once
}

// Events returns a receive-only channel of AgentEvents for this turn.
func (r *Run) Events() <-chan AgentEvent {
	return r.events
}

// Done closes when the turn is complete.
func (r *Run) Done() <-chan struct{} {
	return r.done
}

// Result returns the final RunResult (safe to call after Done closes).
func (r *Run) Result() RunResult {
	r.resultMu.Lock()
	defer r.resultMu.Unlock()
	return r.result
}

// Cancel cancels the turn's session.
func (r *Run) Cancel() {
	r.sess.Cancel()
}

// processProviderEvents transforms raw ProviderEvents into AgentEvents
// and handles tool call execution loop.
func (r *Run) processProviderEvents(ctx context.Context, provEvents <-chan ProviderEvent, req StreamRequest) {
	defer r.doneOnce.Do(func() { close(r.done) })
	defer close(r.events)

	turnID := generateID()
	r.events <- TurnStart{Timestamp_: timeNow(), TurnID: turnID}
	r.sess.EventBus.Emit(Event{Type: EvtTurnStart, Payload: turnID})

	// Fire BeforeAgentStart hook
	if r.sess.Hooks.BeforeAgentStart != nil {
		r.sess.Hooks.BeforeAgentStart.Run(ctx, AgentStartRequest{
			TurnID: turnID,
			Turn:   1,
		})
	}

	var (
		currentMsgID       string
		contentBuf         strings.Builder
		toolCallBuf        = make(map[string]*toolCallAccum)
		pendingToolCalls   []ToolCallRequest
		pendingToolResults []Message // buffered until assistant msg is appended
	)

	for pe := range provEvents {
		select {
		case <-ctx.Done():
			r.events <- TurnEnd{Timestamp_: timeNow(), TurnID: turnID, Reason: string(ReasonCancelled)}
			return
		default:
		}

		switch pe.Type {
		case ProvMessageStart:
			currentMsgID = pe.MessageID
			r.events <- MessageStart{Timestamp_: timeNow(), MessageID: pe.MessageID, Role: RoleAssistant}
			r.sess.EventBus.Emit(Event{Type: EvtMessageStart, Payload: pe.MessageID})

		case ProvContentDelta:
			contentBuf.WriteString(pe.ContentDelta)
			r.events <- MessageDelta{Timestamp_: timeNow(), MessageID: currentMsgID, ContentDelta: pe.ContentDelta}

		case ProvMessageEnd:
			r.events <- MessageEnd{Timestamp_: timeNow(), MessageID: currentMsgID}
			r.sess.EventBus.Emit(Event{Type: EvtMessageEnd, Payload: currentMsgID})

			// Execute pending tools — parallel first, then sequential
			if len(pendingToolCalls) > 0 {
				type toolResult struct {
					callID string
					result ToolResult
					err    error
				}
				results := make([]toolResult, len(pendingToolCalls))

				// Split by execution mode
				type callEntry struct {
					idx int
					tc  ToolCallRequest
				}
				var parallelCalls, sequentialCalls []callEntry
				for i, tc := range pendingToolCalls {
					tool, ok := r.sess.Tools.Get(tc.ToolName)
					if ok && tool.Mode == ModeSequential {
						sequentialCalls = append(sequentialCalls, callEntry{i, tc})
					} else {
						parallelCalls = append(parallelCalls, callEntry{i, tc})
					}
				}

				// Helper: execute a single tool call and store result
				execTool := func(idx int, tc ToolCallRequest) {
					if tool, ok := r.sess.Tools.Get(tc.ToolName); ok {
						// Detect three-phase tool
						if tp := tool.ThreePhase; tp != nil {
							raw := json.RawMessage(tc.Args)
							raw, err := tp.PrepareArgsRaw(raw)
							if err != nil {
								results[idx] = toolResult{tc.CallID, ToolResult{}, err}
								return
							}
							prepared, err := tp.Prepare(ctx, tc.CallID, raw)
							if err != nil {
								results[idx] = toolResult{tc.CallID, ToolResult{}, err}
								return
							}
							result, err := tp.Execute(ctx, prepared, func(pr PartialResult) {
								r.events <- ToolCallUpdate{Timestamp_: timeNow(), CallID: tc.CallID, Partial: pr}
							})
							if finalizeErr := tp.Finalize(ctx, prepared, result); finalizeErr != nil {
								// Log but don't override result error
							}
							results[idx] = toolResult{tc.CallID, result, err}
						} else {
							// Fallback: single-stage Tool.Execute
							var params any
							raw := json.RawMessage(tc.Args)
							if tool.PrepareArgs != nil {
								var err error
								params, err = tool.PrepareArgs(raw)
								if err != nil {
									results[idx] = toolResult{tc.CallID, ToolResult{}, err}
									return
								}
							}
							if params == nil && tc.Args != "" {
								params = raw
							}
							result, err := tool.Execute(ctx, tc.CallID, params, func(pr PartialResult) {
								r.events <- ToolCallUpdate{Timestamp_: timeNow(), CallID: tc.CallID, Partial: pr}
							})
							results[idx] = toolResult{tc.CallID, result, err}
						}
					} else {
						results[idx] = toolResult{tc.CallID, ToolResult{}, fmt.Errorf("tool not found: %s", tc.ToolName)}
					}
				}

				// Execute parallel tools concurrently
				var wg sync.WaitGroup
				for _, ce := range parallelCalls {
					wg.Add(1)
					go func(idx int, tc ToolCallRequest) {
						defer wg.Done()
						execTool(idx, tc)
					}(ce.idx, ce.tc)
				}
				wg.Wait()

				// Execute sequential tools one at a time
				for _, ce := range sequentialCalls {
					execTool(ce.idx, ce.tc)
				}

				// Emit results in deterministic order; buffer for transcript
				for _, tr := range results {
					// Fire AfterToolResult hook
					if r.sess.Hooks.AfterToolResult != nil {
						r.sess.Hooks.AfterToolResult.Run(ctx, ToolResultWithError{
							CallID: tr.callID,
							Result: tr.result,
							Err:    tr.err,
						})
					}
					if tr.err != nil {
						r.events <- ErrorEvent{Timestamp_: timeNow(), Err: tr.err, Code: ErrTool}
					}
					r.events <- ToolCallEnd{
						Timestamp_: timeNow(),
						CallID:     tr.callID,
						Result:     tr.result,
					}
					toolMsg := Message{
						Role:       RoleTool,
						ToolCallID: tr.callID,
						Content:    resultText(tr.result),
						MessageID:  generateMsgID(),
					}
					pendingToolResults = append(pendingToolResults, toolMsg)
				}
			}

			// Append assistant message FIRST (OpenAI requires assistant before tool results)
			assistantMsg := Message{
				Role:      RoleAssistant,
				Content:   contentBuf.String(),
				MessageID: currentMsgID,
				ToolCalls: pendingToolCalls,
			}
			r.sess.Transcript.Append(assistantMsg)
			// THEN append tool results
			for _, tr := range pendingToolResults {
				r.sess.Transcript.Append(tr)
			}
			pendingToolResults = nil

			// Check compaction after each turn
			MaybeCompact(r.sess, DefaultCompactionConfig())

		case ProvToolCallStart:
			if _, ok := toolCallBuf[pe.ToolCallID]; !ok {
				toolCallBuf[pe.ToolCallID] = &toolCallAccum{name: pe.ToolName}
			}
			r.events <- ToolCallStart{
				Timestamp_: timeNow(),
				CallID:     pe.ToolCallID,
				ToolName:   pe.ToolName,
				Args:       nil,
			}
			r.sess.EventBus.Emit(Event{Type: EvtToolCallStart, Payload: map[string]string{"callID": pe.ToolCallID, "name": pe.ToolName}})

		case ProvToolCallDelta:
			if acc, ok := toolCallBuf[pe.ToolCallID]; ok {
				acc.argsBuf.WriteString(pe.ToolArgsDelta)
			}

		case ProvToolCallEnd:
			acc := toolCallBuf[pe.ToolCallID]
			argsJSON := acc.argsBuf.String()

			tcReq := ToolCallRequest{
				CallID:   pe.ToolCallID,
				ToolName: acc.name,
				Args:     argsJSON,
			}
			pendingToolCalls = append(pendingToolCalls, tcReq)

			// Deferred: tools executed in parallel on ProvMessageEnd

		case ProvError:
			r.events <- ErrorEvent{Timestamp_: timeNow(), Err: pe.Err, Code: ErrProvider}
			r.sess.EventBus.Emit(Event{Type: EvtError, Payload: pe.Err.Error()})

		case ProvThinkingDelta:
			r.events <- ThinkingDelta{Timestamp_: timeNow(), Content: pe.ContentDelta}

		case ProvThinkingEnd:
			r.events <- ThinkingEnd{Timestamp_: timeNow()}
		}
	}


	// Determine turn end reason
	reason := ReasonComplete
	if len(pendingToolCalls) > 0 {
		reason = ReasonToolCalls
	}

	// Store result
	r.resultMu.Lock()
	r.result.TurnCount = 1
	if contentBuf.Len() > 0 {
		r.result.FinalMsg = contentBuf.String()
	}
	r.resultMu.Unlock()

	r.sess.EventBus.Emit(Event{Type: EvtTurnEnd, Payload: map[string]string{"reason": string(reason)}})
	r.events <- TurnEnd{Timestamp_: timeNow(), TurnID: turnID, Reason: string(reason)}
}

type toolCallAccum struct {
	name    string
	argsBuf strings.Builder
}

func generateMsgID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}

func timeNow() time.Time {
	return time.Now()
}

// resultText extracts a text representation from a ToolResult.
func resultText(result ToolResult) string {
	for _, c := range result.Content {
		if c.Type == "text" {
			return c.Text
		}
	}
	return ""
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}