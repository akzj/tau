//go:build integration

package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// newIntegrationServer creates a Server with faux provider for integration tests.
func newIntegrationServer(t *testing.T) *Server {
	t.Helper()
	prov := faux.New()
	return New(Options{
		Port:      "0",
		Host:      "localhost",
		Workspace: t.TempDir(),
		Provider:  prov,
		Model:     "test-model",
		WireAPI:   core.WireOpenAICompletions,
	})
}

// TestIntegration_ServerHealth verifies the health endpoint returns 200 with status=ok.
func TestIntegration_ServerHealth(t *testing.T) {
	s := newIntegrationServer(t)

	req := httptest.NewRequest("GET", "/v1/health", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode health response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
	if _, ok := resp["version"]; !ok {
		t.Error("expected version field in health response")
	}
}

// TestIntegration_ChatSSE verifies the SSE streaming endpoint with a faux provider.
func TestIntegration_ChatSSE(t *testing.T) {
	s := newIntegrationServer(t)

	body := bytes.NewBufferString(`{"prompt":"hello world"}`)
	req := httptest.NewRequest("POST", "/v1/chat/stream", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	// SSE endpoint returns 200 even when provider has no queued responses
	// (errors are handled gracefully within the SSE stream)
	if w.Code != 200 {
		t.Fatalf("expected 200 for SSE, got %d: %s", w.Code, w.Body.String())
	}

	// Parse SSE events to verify structure
	scanner := bufio.NewScanner(w.Body)
	eventCount := 0
	hasError := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			eventCount++
			data := strings.TrimPrefix(line, "data: ")

			var evt map[string]string
			if err := json.Unmarshal([]byte(data), &evt); err != nil {
				continue
			}
			if evt["type"] == "error" {
				hasError = true
			}
			t.Logf("SSE event: type=%s", evt["type"])
		}
	}
	t.Logf("SSE stream: %d events received (error=%v)", eventCount, hasError)
}

// TestIntegration_Auth401 verifies 401 when TAU_API_KEYS is set but request has no auth.
func TestIntegration_Auth401(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "integration-test-key-abc123")

	s := newIntegrationServer(t)
	req := httptest.NewRequest("POST", "/v1/sessions", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 401 {
		t.Errorf("expected 401 for missing auth, got %d", w.Code)
	}

	// Verify same endpoint works with valid auth
	req2 := httptest.NewRequest("POST", "/v1/sessions", nil)
	req2.Header.Set("Authorization", "Bearer integration-test-key-abc123")
	w2 := httptest.NewRecorder()
	s.router.ServeHTTP(w2, req2)
	if w2.Code != 201 {
		t.Errorf("expected 201 with valid auth, got %d", w2.Code)
	}
}

// TestIntegration_RateLimit429 verifies rate limiting returns 429 after burst exhaustion.
func TestIntegration_RateLimit429(t *testing.T) {
	t.Setenv("TAU_API_KEYS", "rate-test-key")
	t.Setenv("TAU_RATE_LIMIT", "1") // 1 req/sec, burst 1

	s := newIntegrationServer(t)

	makeReq := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", "/v1/tools", nil)
		req.Header.Set("Authorization", "Bearer rate-test-key")
		w := httptest.NewRecorder()
		s.router.ServeHTTP(w, req)
		return w
	}

	// First request — should pass (burst of 1)
	w1 := makeReq()
	if w1.Code != 200 {
		t.Fatalf("first request: expected 200, got %d", w1.Code)
	}

	// Second request immediately after — should be rate limited
	w2 := makeReq()
	if w2.Code != 429 {
		t.Errorf("second request: expected 429 rate limit, got %d", w2.Code)
	}
}

// TestIntegration_OpenAPI verifies the OpenAPI spec endpoint returns valid 3.0.3 spec.
func TestIntegration_OpenAPI(t *testing.T) {
	s := newIntegrationServer(t)

	req := httptest.NewRequest("GET", "/v1/openapi.json", nil)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200 for openapi, got %d: %s", w.Code, w.Body.String())
	}

	var spec map[string]any
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("failed to parse openapi spec: %v", err)
	}

	// Verify required OpenAPI fields
	if spec["openapi"] != "3.0.3" {
		t.Errorf("expected openapi=3.0.3, got %v", spec["openapi"])
	}

	info, ok := spec["info"].(map[string]any)
	if !ok {
		t.Fatal("missing 'info' section in OpenAPI spec")
	}
	if info["title"] == nil {
		t.Error("missing 'title' in info")
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("missing 'paths' section in OpenAPI spec")
	}

	// Essential paths should be present
	requiredPaths := []string{"/v1/health", "/v1/chat", "/v1/chat/stream", "/v1/sessions"}
	for _, p := range requiredPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing required path %q in OpenAPI spec", p)
		}
	}
}

// TestIntegration_Metrics verifies the Prometheus metrics endpoint.
func TestIntegration_Metrics(t *testing.T) {
	s := newIntegrationServer(t)

	// First hit a few endpoints to generate metrics
	req1 := httptest.NewRequest("GET", "/v1/health", nil)
	w1 := httptest.NewRecorder()
	s.router.ServeHTTP(w1, req1)

	req2 := httptest.NewRequest("GET", "/v1/tools", nil)
	w2 := httptest.NewRecorder()
	s.router.ServeHTTP(w2, req2)

	// Now check metrics endpoint
	req3 := httptest.NewRequest("GET", "/metrics", nil)
	w3 := httptest.NewRecorder()
	s.router.ServeHTTP(w3, req3)

	if w3.Code != 200 {
		t.Fatalf("expected 200 for metrics, got %d: %s", w3.Code, w3.Body.String())
	}

	body := w3.Body.String()
	// Prometheus metrics should contain tau_ prefixed metrics
	expectedMetrics := []string{
		"tau_http_requests_total",
		"tau_http_request_duration_seconds",
	}
	for _, m := range expectedMetrics {
		if !strings.Contains(body, m) {
			t.Errorf("expected metric %q in /metrics output", m)
		}
	}
	t.Logf("metrics endpoint: %d bytes of Prometheus metrics", len(body))
}
