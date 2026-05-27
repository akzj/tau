package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding"
	"github.com/gin-gonic/gin"
)

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok", "version": "0.1.0"})
}

func (s *Server) handleTools(c *gin.Context) {
	if c.Request.Method != "GET" {
		c.AbortWithStatus(405)
		return
	}
	tools := []string{
		"echo", "read", "write", "edit", "bash", "glob", "grep",
		"web_search", "browse", "verify", "git_diff", "git_commit",
		"git_log", "git_branch", "rag_search", "prompt_render",
	}
	c.JSON(200, gin.H{"tools": tools})
}

func (s *Server) handleCreateSession(c *gin.Context) {
	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	sess, err := coding.NewCodingSession(context.Background(), coding.CodingSessionOptions{
		WorkspaceRoot: s.workspace,
		Provider:      s.provider,
		DefaultModel:  core.ModelSpec{Name: s.model, API: s.wireAPI},
	})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	s.mu.Lock()
	s.sessions[id] = sess
	s.mu.Unlock()
	c.JSON(201, gin.H{"id": id})
}

func (s *Server) handleListSessions(c *gin.Context) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.sessions))
	for id := range s.sessions {
		ids = append(ids, id)
	}
	s.mu.RUnlock()
	c.JSON(200, gin.H{"sessions": ids})
}

func (s *Server) handleGetSession(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	sess, err := s.getSession(id)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"id":       id,
		"messages": len(sess.Transcript.Messages()),
	})
}

func (s *Server) handleDeleteSession(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}

	s.mu.Lock()
	delete(s.sessions, id)
	s.mu.Unlock()
	c.JSON(200, gin.H{"deleted": id})
}

func (s *Server) handleChat(c *gin.Context) {
	if c.Request.Method != "POST" {
		c.AbortWithStatus(405)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "invalid json"})
		return
	}
	if req.Prompt == "" {
		c.JSON(400, gin.H{"error": "prompt required"})
		return
	}

	sess, err := s.getOrCreateSession(req.SessionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	loop := core.NewLoop()
	run, err := loop.Prompt(context.Background(), sess.Session, core.UserInput{Text: req.Prompt})
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
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
	c.JSON(200, gin.H{"response": response})
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
