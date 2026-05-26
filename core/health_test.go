package core

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestHealthCheck(t *testing.T) {
	status := HealthCheck(5, 10)
	if status.Status != "ok" {
		t.Errorf("expected 'ok', got %q", status.Status)
	}
	if status.Providers != 5 {
		t.Errorf("expected 5 providers, got %d", status.Providers)
	}
	if status.Tools != 10 {
		t.Errorf("expected 10 tools, got %d", status.Tools)
	}
	if status.Version != Version {
		t.Errorf("expected version %q, got %q", Version, status.Version)
	}
	if status.Uptime == "" {
		t.Error("uptime should not be empty")
	}
}

func TestHealthHandler(t *testing.T) {
	handler := HealthHandler(3, 7)
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var status HealthStatus
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.Providers != 3 {
		t.Errorf("expected 3, got %d", status.Providers)
	}
}

func TestReadyHandler(t *testing.T) {
	handler := ReadyHandler()
	req := httptest.NewRequest("GET", "/ready", nil)
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
