package webui

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/testing/faux"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return NewServer(Options{
		Addr:      ":0",
		Workspace: t.TempDir(),
		Model:     "test-model",
		Provider:  faux.New(),
		WebUI:     true,
	})
}

// TestIndex serves the main chat page.
func TestIndex(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected text/html, got %q", ct)
	}
	body := w.Body.String()
	if !strings.Contains(body, "tau") {
		t.Error("expected tau in index body")
	}
}

// TestChatPage serves the chat page at /chat.
func TestChatPage(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/chat", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "tau") {
		t.Error("expected tau in chat page body")
	}
}

// TestSessionList returns saved sessions.
func TestSessionList(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/sessions", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// Should be valid JSON array or object
	var sessions []struct {
		ID       string `json:"ID"`
		FirstMsg string `json:"FirstMsg"`
	}
	if err := json.NewDecoder(w.Body).Decode(&sessions); err != nil {
		t.Errorf("expected valid JSON sessions, got error: %v", err)
	}
}

// TestSSEChatEmptyPrompt returns 400 for empty prompt.
func TestSSEChatEmptyPrompt(t *testing.T) {
	s := newTestServer(t)
	body := strings.NewReader(`{"session_id":"","prompt":""}`)
	req := httptest.NewRequest("POST", "/v1/chat/sse", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for empty prompt, got %d", w.Code)
	}
}

// TestSSEChatInvalidJSON returns 400 for bad JSON.
func TestSSEChatInvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("POST", "/v1/chat/sse", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Errorf("expected 400 for invalid JSON, got %d", w.Code)
	}
}

// TestSSEChatGET requires prompt query param.
func TestSSEChatGET(t *testing.T) {
	s := newTestServer(t)

	// Empty prompt → 400
	req := httptest.NewRequest("GET", "/v1/chat/stream", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400 for missing prompt, got %d", w.Code)
	}

	// With prompt → 200 SSE response
	req2 := httptest.NewRequest("GET", "/v1/chat/stream?prompt=hello", nil)
	w2 := httptest.NewRecorder()
	s.router.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Errorf("expected 200 for valid prompt, got %d", w2.Code)
	}
	ct := w2.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
}

// TestSSEChatPOST streams SSE data.
func TestSSEChatPOST(t *testing.T) {
	s := newTestServer(t)
	body := strings.NewReader(`{"session_id":"","prompt":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat/sse", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/event-stream") {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
}

// TestOptions validates Options struct.
func TestOptions(t *testing.T) {
	opts := Options{
		Addr:      ":8080",
		Workspace: "/tmp/test",
		Model:     "gpt-4",
		Provider:  faux.New(),
		WebUI:     true,
	}
	if opts.Addr != ":8080" {
		t.Errorf("expected :8080, got %q", opts.Addr)
	}
	if !opts.WebUI {
		t.Error("expected WebUI true")
	}
	if opts.Workspace != "/tmp/test" {
		t.Errorf("expected /tmp/test, got %q", opts.Workspace)
	}
}

// TestEscapeJSONStr validates JSON string escaping.
func TestEscapeJSONStr(t *testing.T) {
	tests := []struct {
		input    string
		contains string
	}{
		{"hello", "hello"},
		{`quo"tes`, `quo`},
		{"new\nline", "new"},
	}
	for _, tt := range tests {
		result := escapeJSONStr(tt.input)
		if !strings.Contains(result, tt.contains) {
			t.Errorf("escapeJSONStr(%q) = %q, expected to contain %q", tt.input, result, tt.contains)
		}
		// Verify it round-trips safely by embedding in JSON
		wrapped := `"` + result + `"`
		var decoded string
		if err := json.Unmarshal([]byte(wrapped), &decoded); err != nil {
			t.Errorf("escapeJSONStr(%q) produced invalid JSON string: %v", tt.input, err)
		}
	}
}

// TestNewServer creates server without panic.
func TestNewServer(t *testing.T) {
	s := NewServer(Options{
		Addr:      ":0",
		Workspace: t.TempDir(),
		Model:     "test",
		Provider:  faux.New(),
		WebUI:     false,
	})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.router == nil {
		t.Fatal("expected non-nil router")
	}
}

// TestGetOrCreateSession creates a new session.
func TestGetOrCreateSession(t *testing.T) {
	s := newTestServer(t)
	sess1, err := s.getOrCreateSession("")
	if err != nil {
		t.Fatalf("getOrCreateSession: %v", err)
	}
	if sess1 == nil {
		t.Fatal("expected non-nil session")
	}

	// Same empty ID creates a new session each time
	sess2, err := s.getOrCreateSession("")
	if err != nil {
		t.Fatalf("second getOrCreateSession: %v", err)
	}
	if sess1 == sess2 {
		t.Error("expected different sessions for empty IDs")
	}
}

// TestWebSocketRoute is registered.
func TestWebSocketRoute(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/ws", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	// WebSocket upgrade failure is expected (no Upgrade header), but route should exist
	if w.Code == 404 {
		t.Error("expected WebSocket route to exist, got 404")
	}
}

// TestHandleIndexServesTemplate validates the embedded template is served.
func TestHandleIndexServesTemplate(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	required := []string{"<!DOCTYPE html>", "tau", "messages", "send"}
	for _, r := range required {
		if !strings.Contains(body, r) {
			t.Errorf("index body missing %q", r)
		}
	}
}
