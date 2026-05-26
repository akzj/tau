//go:build !no_codex

package codex_responses

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

func TestCodexSSEParsing(t *testing.T) {
	sseData := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp-1\",\"output\":[]}}\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello from codex\"}\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp-1\",\"output\":[{\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"hello from codex\"}]}]}}\n" +
		"data: [DONE]"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatal(err)
	}

	var deltas string
	for ev := range events {
		if ev.Type == core.ProvContentDelta {
			deltas += ev.ContentDelta
		}
	}
	if deltas != "hello from codex" {
		t.Errorf("expected 'hello from codex', got %q", deltas)
	}
}

func TestCodexErrorClassification(t *testing.T) {
	tests := []struct {
		status   int
		body     string
		wantErr  string
	}{
		{429, `{"error":{"message":"rate limited"}}`, "rate limited"},
		{500, `{"error":{"message":"internal"}}`, "server error"},
		{401, `{"error":{"message":"unauthorized"}}`, "auth error"},
		{403, `{"error":{"message":"forbidden"}}`, "auth error"},
		{404, `{"error":{"message":"not found"}}`, "API error"},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
			w.Write([]byte(tt.body))
		}))

		prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
		_, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})

		if err == nil {
			srv.Close()
			t.Errorf("status %d: expected error, got nil", tt.status)
			continue
		}
		if !strings.Contains(err.Error(), tt.wantErr) {
			t.Errorf("status %d: expected error containing %q, got %v", tt.status, tt.wantErr, err)
		}
		srv.Close()
	}
}
