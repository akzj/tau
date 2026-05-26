package anthropic_messages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/akzj/tau/core"
)

func TestInterfaceSatisfaction(t *testing.T) {
	os.Setenv("ANTHROPIC_AUTH_TOKEN", "test-token")
	defer os.Unsetenv("ANTHROPIC_AUTH_TOKEN")

	prov, err := NewAnthropicMessagesProvider()
	if err != nil {
		t.Skip("no API key")
	}
	var _ core.Provider = prov
}

func TestErrorOnEmptyAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	_, err := NewAnthropicMessagesProvider()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestAnthropicSSEParsing(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"id":"msg-1","model":"claude"}}
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello from claude"}}
event: message_stop
data: {"type":"message_stop"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &AnthropicMessagesProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6", API: core.WireAnthropicMessages},
	})
	if err != nil {
		t.Fatal(err)
	}

	var deltas string
	for ev := range events {
		if ev.Type == core.ProvContentDelta {
			deltas += ev.ContentDelta
		}
	}
	if deltas != "hello from claude" {
		t.Errorf("expected 'hello from claude', got %q", deltas)
	}
}

func TestAnthropicErrorClassification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"type":"rate_limit_error"}}`))
	}))
	defer srv.Close()

	prov := &AnthropicMessagesProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	_, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6", API: core.WireAnthropicMessages},
	})
	if err == nil {
		t.Error("expected error for 429")
	}
}
func TestAnthropicToolCallSSE(t *testing.T) {
	sseData := `event: message_start
data: {"type":"message_start","message":{"id":"msg-1","model":"claude"}}
event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu-1","name":"echo"}}
event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"msg\":\"hello\"}"}}
event: content_block_stop
data: {"type":"content_block_stop","index":0}
event: message_stop
data: {"type":"message_stop"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &AnthropicMessagesProvider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6", API: core.WireAnthropicMessages},
	})
	if err != nil {
		t.Fatal(err)
	}

	hasToolStart, hasToolEnd, hasToolDelta := false, false, false
	for ev := range events {
		if ev.Type == core.ProvToolCallStart && ev.ToolName == "echo" {
			hasToolStart = true
		}
		if ev.Type == core.ProvToolCallDelta {
			hasToolDelta = true
		}
		if ev.Type == core.ProvToolCallEnd {
			hasToolEnd = true
		}
	}
	if !hasToolStart {
		t.Error("expected tool call start event")
	}
	if !hasToolDelta {
		t.Error("expected tool call delta event")
	}
	if !hasToolEnd {
		t.Error("expected tool call end event")
	}
}