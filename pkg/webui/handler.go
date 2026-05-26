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
		s.sendJSON(conn, serverMsg{Type: "tool_start", CallID: e.CallID, Name: e.ToolName})
	case core.ToolCallEnd:
		content := ""
		for _, c := range e.Result.Content {
			if c.Type == "text" {
				content = c.Text
			}
		}
		s.sendJSON(conn, serverMsg{Type: "tool_end", CallID: e.CallID, Content: content})
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
