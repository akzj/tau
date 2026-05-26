package openai_responses

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

func TestResponsesSSEParsing(t *testing.T) {
	sseData := `data: {"type":"response.created","response":{"id":"resp-1","output":[]}}
data: {"type":"response.output_text.delta","delta":"hello world"}
data: {"type":"response.completed","response":{"id":"resp-1","output":[{"type":"message","content":[{"type":"output_text","text":"hello world"}]}]}}
data: [DONE]`

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
	if deltas != "hello world" {
		t.Errorf("expected 'hello world', got %q", deltas)
	}
}

func TestResponsesErrorClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	_, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err == nil {
		t.Error("expected error for 429")
	}
	if !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestInterfaceSatisfaction(t *testing.T) {
	prov := &Provider{baseURL: "https://example.com/v1", apiKey: "test", client: &http.Client{}}
	var _ core.Provider = prov
}