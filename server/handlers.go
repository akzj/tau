package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok", "version": "0.1.0"})
}

func (s *Server) handleTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	tools := []string{
		"echo", "read", "write", "edit", "bash", "glob", "grep",
		"web_search", "browse", "verify", "git_diff", "git_commit",
		"git_log", "git_branch", "rag_search", "prompt_render",
	}
	writeJSON(w, 200, map[string]any{"tools": tools})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		s.mu.RLock()
		ids := make([]string, 0, len(s.sessions))
		for id := range s.sessions {
			ids = append(ids, id)
		}
		s.mu.RUnlock()
		writeJSON(w, 200, map[string]any{"sessions": ids})
	case "POST":
		id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
		sess, err := coding.NewCodingSession(context.Background(), coding.CodingSessionOptions{
			WorkspaceRoot: s.workspace,
			Provider:      s.provider,
			DefaultModel:  core.ModelSpec{Name: s.model, API: s.wireAPI},
		})
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		s.mu.Lock()
		s.sessions[id] = sess
		s.mu.Unlock()
		writeJSON(w, 201, map[string]string{"id": id})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleSessionByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/v1/sessions/")
	if id == "" {
		http.Error(w, "id required", 400)
		return
	}

	switch r.Method {
	case "GET":
		sess, err := s.getSession(id)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{
			"id":       id,
			"messages": len(sess.Transcript.Messages()),
		})
	case "DELETE":
		s.mu.Lock()
		delete(s.sessions, id)
		s.mu.Unlock()
		writeJSON(w, 200, map[string]string{"deleted": id})
	default:
		http.Error(w, "method not allowed", 405)
	}
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	if req.Prompt == "" {
		writeJSON(w, 400, map[string]string{"error": "prompt required"})
		return
	}

	sess, err := s.getOrCreateSession(req.SessionID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	loop := core.NewLoop()
	run, err := loop.Prompt(context.Background(), sess.Session, core.UserInput{Text: req.Prompt})
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	var response string
	for ev := range run.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			response += msg.ContentDelta
		}
		if _, ok := ev.(core.MessageEnd); ok {
			break
		}
	}
	<-run.Done()
	writeJSON(w, 200, map[string]string{"response": response})
}

func (s *Server) getOrCreateSession(id string) (*coding.CodingSession, error) {
	if id != "" {
		return s.getSession(id)
	}
	newID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	sess, err := coding.NewCodingSession(context.Background(), coding.CodingSessionOptions{
		WorkspaceRoot: s.workspace,
		Provider:      s.provider,
		DefaultModel:  core.ModelSpec{Name: s.model, API: s.wireAPI},
	})
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.sessions[newID] = sess
	s.mu.Unlock()
	return sess, nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
