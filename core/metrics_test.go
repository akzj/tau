package core

import (
	"net/http/httptest"
	"testing"
)

func TestMetricsEndpoint(t *testing.T) {
	RegisterTauMetrics()
	handler := MetricsHandler()
	req := httptest.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	// Verify output contains prometheus metrics (go + process + tau).
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty metrics output")
	}
	if !containsAny(body, "tau_", "go_", "process_") {
		t.Errorf("expected tau_/go_/process_ metrics, got: %s", body[:200])
	}
}

func TestCounterInc(t *testing.T) {
	m := &Metrics{counters: make(map[string]*Counter)}
	c := m.RegisterCounter("test_counter", "help")
	c.Inc()
	if c.Value() != 1 {
		t.Errorf("expected 1, got %d", c.Value())
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
	}
	return false
}