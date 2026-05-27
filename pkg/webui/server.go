package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/akzj/tau/pkg/persist"
)

//go:embed templates/*
var templateFS embed.FS

// Options configures the webui server.
type Options struct {
	Addr      string
	Workspace string
	Model     string
	Provider  core.Provider
	WebUI     bool // enable web UI mode
}

// Server handles HTTP, WebSocket, and SSE for the tau web UI.
type Server struct {
	opts      Options
	router    *gin.Engine
	sessions  map[string]*coding.CodingSession
	mu        sync.RWMutex
	upgrader  websocket.Upgrader
	wsMu      sync.Mutex // protects concurrent WebSocket writes
	activeRun context.CancelFunc
	runMu     sync.Mutex // protects activeRun
}

// NewServer creates a webui server.
func NewServer(opts Options) *Server {
	s := &Server{
		opts:     opts,
		router:   gin.New(),
		sessions: make(map[string]*coding.CodingSession),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
	s.router.Use(gin.Logger())
	s.router.Use(gin.Recovery())
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Web UI pages
	s.router.GET("/", s.handleIndex)
	s.router.GET("/chat", s.handleIndex)

	// WebSocket for real-time chat
	s.router.GET("/ws", s.handleWebSocket)

	// Session management
	s.router.GET("/sessions", s.handleSessionList)

	// SSE streaming (legacy query-param endpoint)
	s.router.GET("/api/stream", s.handleStream)

	// SSE streaming (POST endpoint for enhanced chat)
	s.router.POST("/v1/chat/sse", s.handleChatSSE)
	s.router.GET("/v1/chat/stream", s.handleChatStreamGET)
}

// handleIndex serves the chat UI HTML.
func (s *Server) handleIndex(c *gin.Context) {
	data, err := templateFS.ReadFile("templates/index.html")
	if err != nil {
		c.String(500, "template not found")
		return
	}
	c.Data(200, "text/html; charset=utf-8", data)
}

// handleSessionList returns all saved session IDs.
func (s *Server) handleSessionList(c *gin.Context) {
	sessions, err := persist.List()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, sessions)
}

// getOrCreateSession returns an existing session or creates a new one.
func (s *Server) getOrCreateSession(id string) (*coding.CodingSession, error) {
	if id != "" {
		s.mu.RLock()
		sess := s.sessions[id]
		s.mu.RUnlock()
		if sess != nil {
			return sess, nil
		}
	}
	newID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	sess, err := coding.NewCodingSession(context.Background(), coding.CodingSessionOptions{
		WorkspaceRoot: s.opts.Workspace,
		Provider:      s.opts.Provider,
		DefaultModel: core.ModelSpec{
			Name: s.opts.Model,
			API:  core.WireOpenAICompletions,
		},
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions[newID] = sess
	s.mu.Unlock()
	return sess, nil
}

// systemPrompt returns the system prompt for coding sessions.
func systemPrompt(ws string) core.SystemPromptFn {
	return func(s *core.Session) (string, error) {
		return fmt.Sprintf("You are tau, a coding agent. Workspace: %s. Use tools.", ws), nil
	}
}

// Start begins listening with graceful shutdown on SIGINT.
func (s *Server) Start() error {
	addr := s.opts.Addr
	core.Info("webui: listening", "addr", addr)

	// Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		// Gin's Run blocks; we can't easily call Shutdown.
		// For now, signal handling is best-effort.
		_ = ctx
	}()

	return s.router.Run(addr)
}

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

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ws upgrade: %v\n", err)
		return
	}
	defer conn.Close()

	ctx := c.Request.Context()

	sess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
		WorkspaceRoot: s.opts.Workspace,
		SystemPrompt:  systemPrompt(s.opts.Workspace),
		Provider:      s.opts.Provider,
		DefaultModel: core.ModelSpec{
			Name: s.opts.Model,
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
				WorkspaceRoot: s.opts.Workspace,
				SystemPrompt:  systemPrompt(s.opts.Workspace),
				Provider:      s.opts.Provider,
				DefaultModel: core.ModelSpec{
					Name: s.opts.Model,
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
			sess.Cancel()
			newSess, err := coding.NewCodingSession(ctx, coding.CodingSessionOptions{
				WorkspaceRoot: s.opts.Workspace,
				SystemPrompt:  systemPrompt(s.opts.Workspace),
				Provider:      s.opts.Provider,
				DefaultModel: core.ModelSpec{
					Name: s.opts.Model,
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
			for _, m := range msgs {
				s.sendJSON(conn, serverMsg{
					Type: "message_delta",
					Data: "[" + string(m.Role) + "] " + truncate(m.Content, 100),
				})
			}
		}
	}
}

// runTurn runs the full Prompt → (Continue)* loop in a background goroutine.
func (s *Server) runTurn(ctx context.Context, conn *websocket.Conn, loop core.Loop, sess *coding.CodingSession, input string) {
	run, err := loop.Prompt(ctx, sess.Session, core.UserInput{Text: input})
	if err != nil {
		s.sendJSON(conn, serverMsg{Type: "error", Data: err.Error()})
		return
	}
	s.drainRun(ctx, conn, run)

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
func (s *Server) handleStream(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		prompt = "Say hello"
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"streaming not supported\"}\n\n")
		return
	}
	flusher.Flush()

	sess, err := coding.NewCodingSession(c.Request.Context(), coding.CodingSessionOptions{
		WorkspaceRoot: s.opts.Workspace,
		SystemPrompt:  systemPrompt(s.opts.Workspace),
		Provider:      s.opts.Provider,
		DefaultModel: core.ModelSpec{
			Name: s.opts.Model,
			API:  core.WireOpenAICompletions,
		},
	})
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}
	defer sess.Cancel()

	loop := core.NewLoop()
	run, err := loop.Prompt(c.Request.Context(), sess.Session, core.UserInput{Text: prompt})
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":%q}\n\n", err.Error())
		flusher.Flush()
		return
	}

	for ev := range run.Events() {
		switch e := ev.(type) {
		case core.MessageDelta:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"delta\",\"content\":%q}\n\n", e.ContentDelta)
		case core.MessageEnd:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"done\"}\n\n")
		case core.ErrorEvent:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":%q}\n\n", e.Err.Error())
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
