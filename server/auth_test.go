package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/testing/faux"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	return New(Options{Port: "0", Host: "localhost", Workspace: t.TempDir(), Provider: faux.New()})
}

func handler(t *testing.T, s *Server) *Server {
	t.Helper()
	return s
}

// --- Auth Tests ---

// TestAuthMissingKey_401 verifies 401 is returned when TAU_API_KEYS is set and no key provided.
func TestAuthMissingKey_401(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1,test-key-2")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 for missing key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthValidKeyBearer_200 verifies Bearer token auth works.
func TestAuthValidKeyBearer_200(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	req.Header.Set("Authorization", "Bearer test-key-1")
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 for valid Bearer, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthValidKeyXAPIKey_200 verifies X-API-Key header auth works.
func TestAuthValidKeyXAPIKey_200(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	req.Header.Set("X-API-Key", "test-key-1")
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 for valid X-API-Key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthInvalidKey_401 verifies 401 for wrong key.
func TestAuthInvalidKey_401(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	req.Header.Set("Authorization", "Bearer wrong-key")
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 for invalid key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthQueryParamKey_200 verifies api_key query parameter works.
func TestAuthQueryParamKey_200(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/tools?api_key=test-key-1", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 for query param key, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthDevMode_200 verifies all requests pass when no TAU_API_KEYS is set.
func TestAuthDevMode_200(t *testing.T) {
	// No TAU_API_KEYS env → dev mode
	s := newTestServer(t)
	if !s.keyStore.IsDevMode() {
		t.Fatal("expected dev mode when TAU_API_KEYS is not set")
	}
	req := httptest.NewRequest("GET", "/v1/tools", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200 in dev mode, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthDevMode_HealthBypass verifies /v1/health bypasses auth even with keys set.
func TestAuthDevMode_HealthBypass(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("health should bypass auth, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAuthPublic_OpenAPIBypass verifies /v1/openapi.json is public.
func TestAuthPublic_OpenAPIBypass(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Errorf("openapi should bypass auth, got %d: %s", w.Code, w.Body.String())
	}
}

// --- Admin Key CRUD Tests ---

// TestAdminKey_Unauthorized verifies 401 without admin key.
func TestAdminKey_Unauthorized(t *testing.T) {
	t.Setenv("TAU_ADMIN_KEY", "admin-secret")
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)
	req := httptest.NewRequest("GET", "/v1/admin/keys", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 401 {
		t.Errorf("expected 401 without admin key, got %d", w.Code)
	}
}

// TestAdminKey_CRUD verifies GET, POST, DELETE flow for admin keys.
func TestAdminKey_CRUD(t *testing.T) {
	t.Setenv("TAU_ADMIN_KEY", "admin-secret")
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)

	adminAuth := func(r *http.Request) {
		r.Header.Set("Authorization", "Bearer admin-secret")
	}

	// GET: initially empty (TAU_API_KEYS adds one key)
	req1 := httptest.NewRequest("GET", "/v1/admin/keys", nil)
	adminAuth(req1)
	w1 := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w1, req1)
	if w1.Code != 200 {
		t.Fatalf("GET admin/keys: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}
	var listResp map[string]any
	json.NewDecoder(w1.Body).Decode(&listResp)
	keys := listResp["keys"].([]any)
	if len(keys) != 1 {
		t.Fatalf("expected 1 initial key, got %d", len(keys))
	}

	// POST: create a new key
	req2 := httptest.NewRequest("POST", "/v1/admin/keys", bytes.NewBufferString(`{"key":"custom-key-123"}`))
	adminAuth(req2)
	w2 := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w2, req2)
	if w2.Code != 201 {
		t.Fatalf("POST admin/keys: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var createResp map[string]any
	json.NewDecoder(w2.Body).Decode(&createResp)
	if createResp["key"] != "custom-key-123" {
		t.Errorf("expected key 'custom-key-123', got %v", createResp["key"])
	}
	if createResp["masked"] == nil || !strings.Contains(createResp["masked"].(string), "...") {
		t.Errorf("expected masked key, got %v", createResp["masked"])
	}

	// GET: now 2 keys
	req3 := httptest.NewRequest("GET", "/v1/admin/keys", nil)
	adminAuth(req3)
	w3 := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("GET admin/keys (2): expected 200, got %d", w3.Code)
	}
	var listResp2 map[string]any
	json.NewDecoder(w3.Body).Decode(&listResp2)
	if count, ok := listResp2["count"].(float64); !ok || int(count) != 2 {
		t.Errorf("expected count 2, got %v", listResp2["count"])
	}

	// DELETE: by masked key
	masked := createResp["masked"].(string)
	req4 := httptest.NewRequest("DELETE", "/v1/admin/keys/"+masked, nil)
	adminAuth(req4)
	w4 := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w4, req4)
	if w4.Code != 200 {
		t.Fatalf("DELETE admin/keys: expected 200, got %d: %s", w4.Code, w4.Body.String())
	}

	// GET: back to 1 key
	req5 := httptest.NewRequest("GET", "/v1/admin/keys", nil)
	adminAuth(req5)
	w5 := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w5, req5)
	var listResp3 map[string]any
	json.NewDecoder(w5.Body).Decode(&listResp3)
	if count, ok := listResp3["count"].(float64); !ok || int(count) != 1 {
		t.Errorf("expected count 1 after delete, got %v", listResp3["count"])
	}
}

// TestAdminKey_DeleteNonexistent verifies 404 for deleting unknown key.
func TestAdminKey_DeleteNonexistent(t *testing.T) {
	t.Setenv("TAU_ADMIN_KEY", "admin-secret")
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)

	req := httptest.NewRequest("DELETE", "/v1/admin/keys/nonexistent-key", nil)
	req.Header.Set("Authorization", "Bearer admin-secret")
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("expected 404 for nonexistent key, got %d", w.Code)
	}
}

// --- Rate Limiting Test ---

// TestRateLimit_429 verifies rate limiting returns 429 after burst exhaustion.
func TestRateLimit_429(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	t.Setenv("TAU_RATE_LIMIT", "1") // 1 token/sec, burst 1
	s := newTestServer(t)

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/v1/tools", nil)
		req.Header.Set("Authorization", "Bearer test-key-1")
		w := httptest.NewRecorder()
		s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
		return w
	}

	// First request should pass (burst of 1)
	w1 := makeReq()
	if w1.Code != 200 {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request immediately after — rate limited
	w2 := makeReq()
	if w2.Code != 429 {
		t.Errorf("second request: expected 429 rate limit, got %d", w2.Code)
	}
}

// --- OpenAPI Spec Test ---

// TestOpenAPISpec_Valid verifies /v1/openapi.json returns valid OpenAPI 3.0 spec.
func TestOpenAPISpec_Valid(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "test-key-1")
	s := newTestServer(t)

	req := httptest.NewRequest("GET", "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	s.AuthMiddleware(s.middleware(s.mux)).ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var spec map[string]any
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to parse openapi spec: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi 3.0.3, got %v", spec["openapi"])
	}
	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("missing info section")
	}
	if info["title"] != "tau API" {
		t.Errorf("expected title 'tau API', got %v", info["title"])
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing paths section")
	}

	// Verify key paths exist
	for _, p := range []string{"/v1/health", "/v1/tools", "/v1/sessions", "/v1/chat", "/v1/chat/stream", "/v1/admin/keys"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %s in spec", p)
		}
	}
}