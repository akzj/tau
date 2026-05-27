package core

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// SessionID uniquely identifies a session.
type SessionID string

// SystemPromptFn computes the system prompt at prompt-time.
type SystemPromptFn func(sess *Session) (string, error)

// SessionOptions configures a new Session.
type SessionOptions struct {
	SystemPrompt  SystemPromptFn
	Provider      Provider        // direct provider reference for demo simplicity
	DefaultModel  ModelSpec       // default model for Loop turns
	MaxTokens     int             // token budget (default 128000)
	CtxStrategy   ContextStrategy // context management strategy (default "sliding")
	MemoryDir     string          // directory for memory persistence (empty = disabled)
	ReflectDepth  int             // reflection correction rounds (0 = disabled)
}

// SteerEntry is a pending steer instruction.
type SteerEntry struct {
	Message string
	ID      string
	Time    time.Time
}

// FollowUpEntry is a pending follow-up question.
type FollowUpEntry struct {
	Question string
	ID       string
}

// Session is the unit of isolation. All mutable state lives here.
type Session struct {
	ID            SessionID
	Transcript    *Transcript
	Tools         *ToolRegistry
	Providers     *ProviderRegistry
	Hooks         *HookSet
	SystemPrompt  SystemPromptFn
	Provider      Provider
	DefaultModel  ModelSpec
	TreeEntries   []TreeEntry     // session tree entries
	SteerQueue    []SteerEntry    // pending steer instructions
	followUpQueue []FollowUpEntry // pending follow-up questions
	ActiveTools   []string        // if non-empty, only these tools are sent to LLM
	EventBus      *EventBus       // event subscription system
	Summary       string          // carries compaction summary between turns
	CWD           string          // working directory at session creation
	PendingWrites map[string]string   // path → confirmed content (latest write/edit)
	Conversation  *Conversation       // bounded conversation with token budget
	MaxTokens     int                 // token budget (default 128000)
	CtxStrategy   ContextStrategy     // context management strategy
	TotalUsage    TokenUsage          // cumulative token/cost tracking
	CallCount     int                 // number of LLM calls
	Store         *SessionStore       // persistent store
	CreatedAt     time.Time           // session creation time
	Memory        *MemorySystem       // 3-layer memory system (nil = disabled)
	YesMode       bool                // --yes mode (skip all confirmation)
	Strategy      AgentStrategy       // pluggable reasoning strategy
	Reflection    *ReflectionEngine   // post-turn reflection engine (nil = disabled)
	SkillLoader   *SkillLoader        // skill loader (lazy init in loop)
	StreamUI      chan<- StreamEvent   // nil = non-streaming mode
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewSession creates a new Session.
func NewSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	sessCtx, cancel := context.WithCancel(ctx)
	cwd, _ := os.Getwd()
	s := &Session{
		ID:           SessionID(generateID()),
		Transcript:   NewTranscript(),
		Tools:        NewToolRegistry(),
		Providers:    NewProviderRegistry(),
		Hooks:        &HookSet{},
		SystemPrompt: opts.SystemPrompt,
		Provider:     opts.Provider,
		DefaultModel: opts.DefaultModel,
		EventBus:      NewEventBus(),
		CWD:           cwd,
		PendingWrites: make(map[string]string),
		MaxTokens:     opts.MaxTokens,
		CtxStrategy:   opts.CtxStrategy,
		ctx:           sessCtx,
		cancel:        cancel,
	}
	if s.MaxTokens <= 0 {
		s.MaxTokens = 128000
	}
	if s.CtxStrategy == "" {
		s.CtxStrategy = StrategySliding
	}
	s.Conversation = NewConversation(s.MaxTokens, s.CtxStrategy)
	s.CreatedAt = time.Now()
	if opts.MemoryDir != "" {
		s.Memory = NewMemorySystem(opts.MemoryDir, opts.MemoryDir, nil)
	}
	if s.Strategy == nil {
		s.Strategy = NewReActStrategy()
	}

	// Reflection: subscribe to turn-end events for post-processing
	if opts.ReflectDepth > 0 {
		s.Reflection = NewReflectionEngine(opts.ReflectDepth)
		s.setupReflectionHook()
	}

	return s, nil
}

// Cancel cancels the session's context, signalling all work to stop.
func (s *Session) Cancel() {
	s.cancel()
}

// Save persists the session to its store. No-op if Store is nil.
func (s *Session) Save() error {
	if s.Store == nil {
		return nil
	}
	return s.Store.Save(s)
}

// Context returns the session's context.
func (s *Session) Context() context.Context {
	return s.ctx
}

// ResolveProvider returns the active provider. Checks direct Provider field first,
// then falls back to ProviderRegistry.Get("default").
func (s *Session) ResolveProvider() (Provider, error) {
	if s.Provider != nil {
		return s.Provider, nil
	}
	if p, ok := s.Providers.Get("default"); ok {
		return p, nil
	}
	return nil, fmt.Errorf("no provider configured")
}

// generateID creates a simple unique ID (demo quality).
func generateID() string {
	return fmt.Sprintf("sess-%d", time.Now().UnixNano())
}

// AddEntry fires BeforeSessionTree hook and appends the entry to the tree.
func (s *Session) AddEntry(entry TreeEntry) {
	if s.Hooks.BeforeSessionTree != nil {
		s.Hooks.BeforeSessionTree.Run(s.ctx, entry)
	}
	s.TreeEntries = append(s.TreeEntries, entry)
}

// Steer injects a direction instruction into the session. Consumed next turn.
func (s *Session) Steer(message string) SteerEntry {
	entry := SteerEntry{
		Message: message,
		ID:      generateID(),
		Time:    time.Now(),
	}
	s.SteerQueue = append(s.SteerQueue, entry)
	return entry
}

// FollowUp injects a follow-up question.
func (s *Session) FollowUp(question string) FollowUpEntry {
	entry := FollowUpEntry{
		Question: question,
		ID:       generateID(),
	}
	s.followUpQueue = append(s.followUpQueue, entry)
	return entry
}

// DrainSteers consumes all pending steer entries and returns them as a system message.
func (s *Session) DrainSteers() ([]SteerEntry, string) {
	if len(s.SteerQueue) == 0 {
		return nil, ""
	}
	entries := s.SteerQueue
	s.SteerQueue = nil
	var parts []string
	for _, e := range entries {
		parts = append(parts, e.Message)
	}
	return entries, strings.Join(parts, "\n")
}

// SetTools restricts which tools the LLM can see.
// Pass nil or empty to restore all registered tools.
func (s *Session) SetTools(names []string) {
	s.ActiveTools = names
}

// GetActiveTools returns the current active tool list (nil means all).
func (s *Session) GetActiveTools() []string {
	return s.ActiveTools
}

// On subscribes to a specific event type. Returns a cancel function.
func (s *Session) On(eventType EventType, handler func(Event)) func() {
	ch, cancel := s.EventBus.Subscribe()
	go func() {
		for evt := range ch {
			if evt.Type == eventType {
				handler(evt)
			}
		}
	}()
	return cancel
}

// setupReflectionHook subscribes to turn-end events and runs post-turn reflection.
// When issues are detected, a correction prompt is injected as a user message
// into the conversation so the next turn picks it up.
func (s *Session) setupReflectionHook() {
	if s.Reflection == nil {
		return
	}
	ch, cancel := s.EventBus.Subscribe()
	go func() {
		defer cancel()
		for evt := range ch {
			if evt.Type != EvtTurnEnd || s.Reflection == nil {
				continue
			}
			// Find last assistant message in the conversation
			msgs := s.Conversation.ToMessages()
			var lastContent string
			for i := len(msgs) - 1; i >= 0; i-- {
				if msgs[i].Role == RoleAssistant && msgs[i].Content != "" {
					lastContent = msgs[i].Content
					break
				}
			}
			if lastContent == "" {
				continue
			}

			result := s.Reflection.Review(lastContent)
			if result.Passed {
				continue
			}

			correction := s.Reflection.BuildCorrectionPrompt(lastContent, result.Issues)
			correctionMsg := Message{
				Role:    RoleUser,
				Content: correction,
			}
			s.Conversation.Add(correctionMsg)
			s.Transcript.Append(correctionMsg)
		}
	}()
}
