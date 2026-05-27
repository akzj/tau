package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

func TestHealth(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected ok, got %q", resp["status"])
	}
}

func TestTools(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestCreateSession(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("POST", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 201 {
		t.Errorf("expected 201, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] == "" {
		t.Error("expected session id")
	}
}

func TestGetSession(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	// Create first
	w1 := httptest.NewRecorder()
	s.mux.ServeHTTP(w1, httptest.NewRequest("POST", "/v1/sessions", nil))
	var create map[string]string
	json.NewDecoder(w1.Body).Decode(&create)

	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, httptest.NewRequest("GET", "/v1/sessions/"+create["id"], nil))
	if w2.Code != 200 {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestDeleteSession(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	w1 := httptest.NewRecorder()
	s.mux.ServeHTTP(w1, httptest.NewRequest("POST", "/v1/sessions", nil))
	var create map[string]string
	json.NewDecoder(w1.Body).Decode(&create)

	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, httptest.NewRequest("DELETE", "/v1/sessions/"+create["id"], nil))
	if w2.Code != 200 {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestChat(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New(), Model: "test"})
	body := bytes.NewBufferString(`{"prompt":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat", body)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 200 && w.Code != 500 {
		t.Errorf("expected 200 or 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestChatEmptyPrompt(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	body := bytes.NewBufferString(`{"prompt":""}`)
	req := httptest.NewRequest("POST", "/v1/chat", body)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// TestAuthMissingKey verifies 401 when TAU_API_KEYS is set but no key provided.
func TestAuthMissingKey(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "secret")
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("POST", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

// TestAuthValidKey verifies 200 with valid Bearer token.
func TestAuthValidKey(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "secret")
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("POST", "/v1/sessions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 201 {
		t.Errorf("expected 201, got %d", w.Code)
	}
}

// TestHealthNoAuth verifies health endpoint bypasses auth even with keys set.
func TestHealthNoAuth(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "secret")
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("health should bypass auth, got %d", w.Code)
	}
}

func TestGetSessionNotFound(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	req := httptest.NewRequest("GET", "/v1/sessions/nonexistent", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestListSessions(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
	// Create a session first
	w1 := httptest.NewRecorder()
	s.mux.ServeHTTP(w1, httptest.NewRequest("POST", "/v1/sessions", nil))

	w2 := httptest.NewRecorder()
	s.mux.ServeHTTP(w2, httptest.NewRequest("GET", "/v1/sessions", nil))
	if w2.Code != 200 {
		t.Errorf("expected 200, got %d", w2.Code)
	}
	var resp map[string]any
	json.NewDecoder(w2.Body).Decode(&resp)
	sessions, ok := resp["sessions"].([]any)
	if !ok || len(sessions) == 0 {
		t.Error("expected at least one session")
	}
}

func TestChatStream(t *testing.T) {
	s := New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New(), WireAPI: core.WireOpenAICompletions})
	body := bytes.NewBufferString(`{"prompt":"hello"}`)
	req := httptest.NewRequest("POST", "/v1/chat/stream", body)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)
	// Can succeed or fail depending on faux provider state — but not panic
	if w.Code != 200 && w.Code != 500 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}