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
}

func TestCounterInc(t *testing.T) {
	m := &Metrics{counters: make(map[string]*counterMetric)}
	c := m.RegisterCounter("test_counter", "help")
	c.Inc()
}
