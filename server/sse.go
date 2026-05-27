package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/akzj/tau/core"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleChatStream(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.SSEvent("error", gin.H{"type": "error", "message": "invalid json"})
		return
	}
	if req.Prompt == "" {
		c.SSEvent("error", gin.H{"type": "error", "message": "prompt required"})
		return
	}

	sess, err := s.getOrCreateSession(req.SessionID)
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(err.Error()))
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
	run, err := loop.Prompt(context.Background(), sess.Session, core.UserInput{Text: req.Prompt})
	if err != nil {
		fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(err.Error()))
		flusher.Flush()
		return
	}

	for ev := range run.Events() {
		switch msg := ev.(type) {
		case core.MessageDelta:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"delta\",\"content\":\"%s\"}\n\n", escapeJSON(msg.ContentDelta))
			flusher.Flush()
		case core.MessageEnd:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"done\"}\n\n")
			flusher.Flush()
		case core.ErrorEvent:
			fmt.Fprintf(c.Writer, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(msg.Err.Error()))
			flusher.Flush()
		case core.TurnEnd:
			break
		}
	}
	<-run.Done()
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
