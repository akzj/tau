package webui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/akzj/tau/core"
)

// handleChatSSE handles SSE streaming chat via POST.
// Accepts JSON body with "session_id" and "prompt".
func (s *Server) handleChatSSE(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	if req.Prompt == "" {
		c.JSON(400, gin.H{"error": "prompt required"})
		return
	}

	s.serveSSE(c, req.SessionID, req.Prompt)
}

// handleChatStreamGET handles SSE streaming chat via GET (for EventSource).
func (s *Server) handleChatStreamGET(c *gin.Context) {
	prompt := c.Query("prompt")
	if prompt == "" {
		c.JSON(400, gin.H{"error": "prompt required"})
		return
	}
	sessionID := c.Query("session_id")
	s.serveSSE(c, sessionID, prompt)
}

// serveSSE is the shared SSE streaming implementation.
func (s *Server) serveSSE(c *gin.Context, sessionID, prompt string) {
	sess, err := s.getOrCreateSession(sessionID)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSONStr(err.Error()))
		c.Writer.Flush()
		return
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

	loop := core.NewLoop()
	run, err := loop.Prompt(context.Background(), sess.Session, core.UserInput{Text: prompt})
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSONStr(err.Error()))
		flusher.Flush()
		return
	}

	for ev := range run.Events() {
		switch e := ev.(type) {
		case core.MessageDelta:
			data, _ := json.Marshal(map[string]string{"type": "delta", "content": e.ContentDelta})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		case core.ToolCallStart:
			data, _ := json.Marshal(map[string]string{"type": "tool_start", "tool": e.ToolName})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		case core.ToolCallEnd:
			data, _ := json.Marshal(map[string]string{"type": "tool_end", "call_id": e.CallID})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		case core.MessageEnd:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"done\"}\n\n")
		case core.ErrorEvent:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSONStr(e.Err.Error()))
		case core.TurnEnd:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"turn_end\"}\n\n")
		case core.ThinkingDelta:
			data, _ := json.Marshal(map[string]string{"type": "thinking", "content": e.Content})
			fmt.Fprintf(c.Writer, "data: %s\n\n", data)
		case core.ThinkingEnd:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"thinking_end\"}\n\n")
		}
		flusher.Flush()
	}
	<-run.Done()
}

// escapeJSONStr escapes a string for safe inclusion in JSON.
func escapeJSONStr(s string) string {
	b, _ := json.Marshal(s)
	if len(b) >= 2 {
		return string(b[1 : len(b)-1])
	}
	return s
}
