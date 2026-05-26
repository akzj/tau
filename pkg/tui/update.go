package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/persist"
	"github.com/akzj/tau/pkg/provider"
)

// Update implements tea.Model.
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.session.Cancel()
			return m, tea.Quit

		case "ctrl+l":
			m.messages = nil
			m.streaming = ""
			m.tools = make(map[string]toolState)
			m.turnCount = 0
			return m, nil

		case "ctrl+s":
			id := persist.NewID()
			msgs := m.session.Transcript.Messages()
			if err := persist.Save(id, msgs); err == nil {
				m.messages = append(m.messages, line{Role: "system", Content: fmt.Sprintf("session saved: %s (%d msgs)", id, len(msgs))})
			} else {
				m.messages = append(m.messages, line{Role: "system", Content: fmt.Sprintf("save failed: %v", err)})
			}
			return m, nil

		case "ctrl+m":
			modelReg, _ := provider.LoadModelRegistry()
			var lines []string
			for _, info := range modelReg.List("") {
				lines = append(lines, fmt.Sprintf("%s (%s) %d ctx", info.ID, info.Provider, info.ContextWindow))
			}
			m.messages = append(m.messages, line{Role: "system", Content: "Models:\n" + strings.Join(lines, "\n")})
			return m, nil

		case "ctrl+t":
			m.showFiles = !m.showFiles
			return m, nil

		case "enter":
			input := m.input.Value()
			if input == "" {
				return m, nil
			}
			m.input.Reset()
			m.messages = append(m.messages, line{Role: "user", Content: input})
			m.status = "streaming"
			m.streaming = ""
			m.tools = make(map[string]toolState)
			m.err = nil

			// Start agent turn loop in background goroutine.
			go m.runTurnLoop(input)
			// Continue listening — agent events will arrive via agentChan.
			return m, listenEvents(m.agentChan)

		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.input.Width = msg.Width - 4
		if m.input.Width < 10 {
			m.input.Width = 10
		}
		return m, nil

	case eventMsg:
		return m, m.handleAgentEvent(msg.event)

	case turnCompleteMsg:
		if msg.err != nil {
			m.messages = append(m.messages, line{Role: "system", Content: "error: " + msg.err.Error()})
			m.err = msg.err
		}
		m.status = "idle"
		// Keep listening for future events (e.g. from next user input).
		return m, listenEvents(m.agentChan)
	}

	return m, nil
}

// handleAgentEvent processes a single AgentEvent and returns the next Cmd.
func (m *model) handleAgentEvent(ev core.AgentEvent) tea.Cmd {
	switch e := ev.(type) {
	case core.MessageStart:
		m.streaming = ""

	case core.MessageDelta:
		m.streaming += e.ContentDelta

	case core.MessageEnd:
		if m.streaming != "" {
			m.messages = append(m.messages, line{Role: "assistant", Content: m.streaming})
			m.streaming = ""
		}

	case core.ToolCallStart:
		m.tools[e.CallID] = toolState{Name: e.ToolName, Status: "running", Args: e.Args}

	case core.ToolCallEnd:
		ts := m.tools[e.CallID]
		ts.Status = "done"
		for _, c := range e.Result.Content {
			if c.Type == "text" {
				ts.Result = c.Text
				break
			}
		}
		m.tools[e.CallID] = ts
		m.messages = append(m.messages, line{Role: "tool", Content: "  [" + ts.Name + "] → " + truncate(ts.Result, 200)})

		// Update file tree from tool result.
		if path, ok := extractPath(ts.Name, ts.Args, ts.Result); ok {
			switch ts.Name {
			case "read":
				m.fileTree.Mark(path, FileRead)
			case "write":
				m.fileTree.Mark(path, FileCreated)
			case "edit":
				m.fileTree.Mark(path, FileModified)
			}
		}

	case core.TurnStart:
		m.status = "streaming"

	case core.TurnEnd:
		m.turnCount++
		if e.Reason == "complete" {
			m.status = "idle"
		}
		// "tool_calls" → keep streaming (Continue will pick up)

	case core.ErrorEvent:
		m.messages = append(m.messages, line{Role: "system", Content: "error: " + e.Err.Error()})

	case core.ThinkingDelta:
		m.thinkingBuf.WriteString(e.Content)

	case core.ThinkingEnd:
		if m.thinkingBuf.Len() > 0 {
			m.messages = append(m.messages, line{Role: "thinking", Content: m.thinkingBuf.String()})
			m.thinkingBuf.Reset()
		}
	}

	// Continue listening for more events.
	return listenEvents(m.agentChan)
}

// runTurnLoop runs the full Prompt → (Continue)* loop in a background goroutine.
// All AgentEvents are pumped into agentChan. When done, turnCompleteMsg is sent.
func (m *model) runTurnLoop(input string) {
	ctx := m.session.Context()

	// First turn: Prompt
	run, err := m.loop.Prompt(ctx, m.session.Session, core.UserInput{Text: input})
	if err != nil {
		m.agentChan <- turnCompleteMsg{err: err}
		return
	}

	// Drain events from first run.
	m.drainRun(ctx, run)

	// Continue loop: keep going while tool calls were made.
	for {
		// We need to call Continue. But we need to know if the last turn had tool calls.
		// Check: if the last message in transcript has tool calls, continue.
		msgs := m.session.Transcript.Messages()
		if len(msgs) == 0 {
			break
		}
		last := msgs[len(msgs)-1]
		// If the last message is a tool result, we should continue.
		// If it's an assistant message without tool calls, we're done.
		if last.Role == core.RoleTool {
			// There was a tool result — continue.
			run, err = m.loop.Continue(ctx, m.session.Session)
			if err != nil {
				m.agentChan <- turnCompleteMsg{err: err}
				return
			}
			m.drainRun(ctx, run)
		} else if last.Role == core.RoleAssistant && len(last.ToolCalls) > 0 {
			// Assistant wants tool calls — but we just drained a run.
			// This means tool execution happened and we should continue.
			run, err = m.loop.Continue(ctx, m.session.Session)
			if err != nil {
				m.agentChan <- turnCompleteMsg{err: err}
				return
			}
			m.drainRun(ctx, run)
		} else {
			break
		}
	}

	m.agentChan <- turnCompleteMsg{}
}

// drainRun reads all AgentEvents from a Run and forwards them to agentChan.
// Blocks until the run completes or context is cancelled.
func (m *model) drainRun(ctx context.Context, run *core.Run) {
	for {
		select {
		case ev, ok := <-run.Events():
			if !ok {
				return
			}
			m.agentChan <- ev
		case <-ctx.Done():
			return
		}
	}
}

// extractPath extracts a file path from tool call args and result text.
func extractPath(toolName string, args json.RawMessage, result string) (string, bool) {
	switch toolName {
	case "read":
		var a struct{ FilePath string `json:"file_path"` }
		if json.Unmarshal(args, &a) == nil && a.FilePath != "" {
			return a.FilePath, true
		}
	case "write":
		// "Wrote N bytes to <path>" or "Wrote N bytes to <path> (overwritten)"
		if after := afterLast(result, " to "); after != "" {
			path := strings.TrimSuffix(after, " (overwritten)")
			path = strings.TrimSuffix(path, " (empty — file cleared)")
			return path, true
		}
	case "edit":
		// "Replaced N occurrence(s) in <path>"
		if after := afterLast(result, " in "); after != "" {
			return after, true
		}
	}
	return "", false
}

// afterLast returns the substring after the last occurrence of sep.
func afterLast(s, sep string) string {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return ""
	}
	return s[idx+len(sep):]
}

// truncate shortens s to maxLen characters, appending "…" if needed.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
