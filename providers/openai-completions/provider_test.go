package openai_completions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

func TestInterfaceSatisfaction(t *testing.T) {
	os.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
	defer os.Unsetenv("ANTHROPIC_AUTH_TOKEN")

	prov, err := NewOpenAICompletionsProvider()
	if err != nil {
		t.Skip("no API key")
	}
	var _ core.Provider = prov
}

func TestErrorOnEmptyAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	_, err := NewOpenAICompletionsProvider()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestOpenAISSEParsing(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"hello "},"index":0}]}
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"content":"world"},"index":0}]}
data: [DONE]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &OpenAICompletionsProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
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

func TestOpenAIErrorClassification(t *testing.T) {
	tests := []struct {
		code     int
		contains string
	}{
		{429, "rate limited"},
		{401, "auth error"},
		{403, "auth error"},
		{500, "server error"},
		{503, "server error"},
	}
	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.code)
			w.Write([]byte(`{"error":{"message":"test"}}`))
		}))
		prov := &OpenAICompletionsProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
		_, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
		srv.Close()
		if err == nil {
			t.Errorf("expected error for status %d", tt.code)
		}
		if !strings.Contains(err.Error(), tt.contains) {
			t.Errorf("status %d: expected %q in error, got %v", tt.code, tt.contains, err)
		}
	}
}

func TestOpenAIToolCallSSE(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-xxx","choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"echo","arguments":""}}]},"index":0}]}
data: {"id":"chatcmpl-xxx","choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"msg\":\"hello\"}"}}]},"index":0}]}
data: {"id":"chatcmpl-xxx","choices":[{"finish_reason":"tool_calls"}],"index":0}
data: [DONE]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &OpenAICompletionsProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatal(err)
	}

	hasToolStart, hasToolEnd := false, false
	for ev := range events {
		if ev.Type == core.ProvToolCallStart {
			hasToolStart = true
		}
		if ev.Type == core.ProvToolCallEnd {
			hasToolEnd = true
		}
	}
	if !hasToolStart || !hasToolEnd {
		t.Error("expected tool call start+end events")
	}
}