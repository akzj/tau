package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/akzj/tau/core"
)

func (s *Server) handleChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"invalid json\"}\n\n")
		return
	}
	if req.Prompt == "" {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"prompt required\"}\n\n")
		return
	}

	sess, err := s.getOrCreateSession(req.SessionID)
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(err.Error()))
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	loop := core.NewLoop()
	run, err := loop.Prompt(context.Background(), sess.Session, core.UserInput{Text: req.Prompt})
	if err != nil {
		fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(err.Error()))
		flusher.Flush()
		return
	}

	for ev := range run.Events() {
		switch msg := ev.(type) {
		case core.MessageDelta:
			fmt.Fprintf(w, "data: {\"type\":\"delta\",\"content\":\"%s\"}\n\n", escapeJSON(msg.ContentDelta))
			flusher.Flush()
		case core.MessageEnd:
			fmt.Fprintf(w, "data: {\"type\":\"done\"}\n\n")
			flusher.Flush()
		case core.ErrorEvent:
			fmt.Fprintf(w, "data: {\"type\":\"error\",\"message\":\"%s\"}\n\n", escapeJSON(msg.Err.Error()))
			flusher.Flush()
		case core.TurnEnd:
			break
		}
	}
}

func escapeJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}
