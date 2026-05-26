package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/websocket"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/persist"
)

// wsMsg is a message received from the browser.
type wsMsg struct {
	Type string `json:"type"` // "prompt"
	Text string `json:"text,omitempty"`
}

// serverMsg is a message sent to the browser.
type serverMsg struct {
	Type        string         `json:"type"`
	Data        string         `json:"data,omitempty"`
	CallID      string         `json:"call_id,omitempty"`
	Name        string         `json:"name,omitempty"`
	Content     string         `json:"content,omitempty"`
	Thinking    string         `json:"thinking,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
	ActiveTools []string       `json:"active_tools,omitempty"`
	FilePath    string         `json:"file_path,omitempty"`
	FileAction  string         `json:"file_action,omitempty"`
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ws upgrade: %v\n", err)
		return
	}
	defer conn.Close()

	ctx := r.Context()

	sess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
		WorkspaceRoot: s.workspace,
		SystemPrompt:  systemPrompt(s.workspace),
		Provider:      s.provider,
		DefaultModel: core.ModelSpec{
			Name: s.model,
			API:  core.WireOpenAICompletions,
		},
	})
	if err != nil {
		s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
		return
	}
	defer sess.Cancel()

	loop := core.NewLoop()

	// Send session ID on connect.
	s.sendJSON(conn, serverMsg{Type: "session", Data: persist.NewID()})

	// Read messages from the browser.
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var msg wsMsg
		json.Unmarshal(raw, &msg)

		switch msg.Type {
		case "prompt":
			if msg.Text != "" {
				s.runMu.Lock()
				if s.activeRun != nil {
					s.activeRun() // cancel previous turn
				}
				turnCtx, turnCancel := context.WithCancel(ctx)
				s.activeRun = turnCancel
				s.runMu.Unlock()
				go s.runTurn(turnCtx, conn, loop, sess, msg.Text)
			}
		case "new_session":
			sess.Cancel()
			newSess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
				WorkspaceRoot: s.workspace,
				SystemPrompt:  systemPrompt(s.workspace),
				Provider:      s.provider,
				DefaultModel: core.ModelSpec{
					Name: s.model,
					API:  core.WireOpenAICompletions,
				},
			})
			if err != nil {
				s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
				break
			}
			sess = newSess
			s.sendJSON(conn, serverMsg{Type: "session", Data: persist.NewID()})
		case "resume":
			msgs, err := persist.Load(msg.Text)
			if err != nil {
				s.sendJSON(conn, serverMsg{Type: "error", Data: fmt.Sprintf("load session: %v", err)})
				break
			}
			// Cancel old session and create new one for clean state.
			sess.Cancel()
			newSess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
				WorkspaceRoot: s.workspace,
				SystemPrompt:  systemPrompt(s.workspace),
				Provider:      s.provider,
				DefaultModel: core.ModelSpec{
					Name: s.model,
					API:  core.WireOpenAICompletions,
				},
			})
			if err != nil {
				s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
				break
			}
			sess = newSess
			sess.Transcript.Append(msgs...)
			s.sendJSON(conn, serverMsg{Type: "session", Data: msg.Text})
			// Replay messages as system info.
			for _, m := range msgs {
				s.sendJSON(conn, serverMsg{
					Type: "message_delta",
					Data: "[" + string(m.Role) + "] " + truncate(m.Content, 100),
				})
			}
		}
	}
}

func systemPrompt(ws string) core.SystemPromptFn {
	return func(s *core.Session) (string, error) {
		return fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools.", ws), nil
	}
}

// runTurn runs the full Prompt → (Continue)* loop in a background goroutine.
// AgentEvents are streamed as JSON over the WebSocket connection.
func (s *Server) runTurn(ctx context.Context, conn *websocket.Conn, loop core.Loop, sess *coding.CodingSession, input string) {
	// First turn: Prompt
	run, err := loop.Prompt(ctx, sess.Session, core.UserInput{Text: input})
	if err != nil {
		s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
		return
	}
	s.drainRun(ctx, conn, run)

	// Continue loop: keep going while tool calls were made.
	for {
		msgs := sess.Transcript.Messages()
		if len(msgs) == 0 {
			break
		}
		last := msgs[len(msgs)-1]
		if last.Role == core.RoleTool || (last.Role == core.RoleAssistant && len(last.ToolCalls) > 0) {
			run, err = loop.Continue(ctx, sess.Session)
			if err != nil {
				s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
				return
			}
			s.drainRun(ctx, conn, run)
		} else {
			break
		}
	}
}

// drainRun reads all AgentEvents from a Run and sends them as JSON over the WebSocket.
func (s *Server) drainRun(ctx context.Context, conn *websocket.Conn, run *core.Run) {
	for {
		select {
		case ev, ok := <-run.Events():
			if !ok {
				return
			}
			s.sendEvent(conn, ev)
		case <-ctx.Done():
			return
		}
	}
}

// sendEvent converts an AgentEvent to a serverMsg and sends it over the WebSocket.
func (s *Server) sendEvent(conn *websocket.Conn, ev core.AgentEvent) {
	switch e := ev.(type) {
	case core.MessageStart:
		s.sendJSON(conn, serverMsg{Type: "message_start", CallID: e.MessageID})
	case core.MessageDelta:
		s.sendJSON(conn, serverMsg{Type: "message_delta", Data: e.ContentDelta})
	case core.MessageEnd:
		s.sendJSON(conn, serverMsg{Type: "message_end"})
	case core.ToolCallStart:
		msg := serverMsg{Type: "tool_start", CallID: e.CallID, Name: e.ToolName}
		// Try to extract file path from Args for read/write/edit tools.
		if e.ToolName == "read" || e.ToolName == "write" || e.ToolName == "edit" || e.ToolName == "glob" || e.ToolName == "grep" {
			var args map[string]any
			if json.Unmarshal(e.Args, &args) == nil {
				if path, ok := args["file_path"].(string); ok {
					msg.FilePath = path
					msg.FileAction = e.ToolName
				} else if path, ok := args["path"].(string); ok {
					msg.FilePath = path
					msg.FileAction = e.ToolName
				}
			}
		}
		s.sendJSON(conn, msg)
	case core.ToolCallEnd:
		content := ""
		for _, c := range e.Result.Content {
			if c.Type == "text" {
				content = c.Text
			}
		}
		msg := serverMsg{Type: "tool_end", CallID: e.CallID, Content: content}
		// Extract file info from Result.Details.
		if e.Result.Details != nil {
			if path, ok := e.Result.Details["path"].(string); ok {
				msg.FilePath = path
			} else if path, ok := e.Result.Details["file_path"].(string); ok {
				msg.FilePath = path
			}
			if action, ok := e.Result.Details["action"].(string); ok {
				msg.FileAction = action
			} else if msg.FilePath != "" {
				msg.FileAction = "modified"
			}
		}
		s.sendJSON(conn, msg)
	case core.TurnEnd:
		s.sendJSON(conn, serverMsg{Type: "turn_end", Data: e.Reason})
	case core.ErrorEvent:
		s.sendJSON(conn, serverMsg{Type: "error", Data: e.Err.Error()})
	case core.ThinkingDelta:
		s.sendJSON(conn, serverMsg{Type: "thinking_delta", Data: e.Content})
	case core.ThinkingEnd:
		s.sendJSON(conn, serverMsg{Type: "thinking_end"})
	}
}

// handleStream serves SSE (Server-Sent Events) for real-time agent output.
// Uses run.Events() (AgentEvent stream) rather than StreamUI, since Prompt
// always goes through processProviderEvents which does not write to StreamUI.
func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	prompt := r.URL.Query().Get("prompt")
	if prompt == "" {
		prompt = "Say hello"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	sess, err := coding.NewCodingSession(r.Context(), coding.CodingSessionOptions{
		WorkspaceRoot: s.workspace,
		SystemPrompt:  systemPrompt(s.workspace),
		Provider:      s.provider,
		DefaultModel: core.ModelSpec{
			Name: s.model,
			API:  core.WireOpenAICompletions,
		},
	})
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer sess.Cancel()

	loop := core.NewLoop()
	run, err := loop.Prompt(r.Context(), sess.Session, core.UserInput{Text: prompt})
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	for ev := range run.Events() {
		switch e := ev.(type) {
		case core.MessageDelta:
			fmt.Fprintf(w, "data: {\"type\":\"delta\",\"content\":%q}\n\n", e.ContentDelta)
		case core.MessageEnd:
			fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
		case core.ErrorEvent:
			fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":%q}\n\n", e.Err.Error())
		}
		flusher.Flush()
	}
}

func (s *Server) sendJSON(conn *websocket.Conn, msg serverMsg) {
	s.wsMu.Lock()
	defer s.wsMu.Unlock()
	conn.WriteJSON(msg)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
