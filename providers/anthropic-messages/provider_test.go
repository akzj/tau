//go:build !no_anthropic

package anthropic_messages

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

// --- Backward-compat tests ---------------------------------------------------

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

// --- Requirement 1: Text completion ------------------------------------------

func TestCompleteText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "hello world"}},
			"usage":   map[string]int{"input_tokens": 10, "output_tokens": 5},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 5 {
		t.Errorf("expected 5 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("expected 15 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

// --- Requirement 2: Streaming ------------------------------------------------

func TestStreamText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\"}}\n\n"))
		w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n"))
		w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		w.Write([]byte("event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"}}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var text string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
}

// --- Requirement 3: Vision (image content blocks) ----------------------------

func TestVisionImage(t *testing.T) {
	// Verify the provider sends type:image + base64 source in content[].
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "image analyzed"}},
			"usage":   map[string]int{"input_tokens": 100, "output_tokens": 10},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())

	// Build a message with vision content blocks (JSON-encoded in Content).
	visionBlocks := `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"base64data"}},{"type":"text","text":"describe this image"}]`
	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: visionBlocks}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "image analyzed" {
		t.Errorf("expected 'image analyzed', got %q", resp.Content)
	}

	// Verify the request body had proper image content blocks.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	msgs := reqBody["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)

	// First block should be the image.
	img := content[0].(map[string]any)
	if img["type"] != "image" {
		t.Errorf("expected content[0].type = 'image', got %q", img["type"])
	}
	source := img["source"].(map[string]any)
	if source["type"] != "base64" {
		t.Errorf("expected source.type = 'base64', got %q", source["type"])
	}
	if source["media_type"] != "image/png" {
		t.Errorf("expected source.media_type = 'image/png', got %q", source["media_type"])
	}
	if source["data"] != "base64data" {
		t.Errorf("expected source.data = 'base64data', got %q", source["data"])
	}

	// Second block should be the text.
	txt := content[1].(map[string]any)
	if txt["type"] != "text" {
		t.Errorf("expected content[1].type = 'text', got %q", txt["type"])
	}
}

// --- Requirement 4: Tool Use -------------------------------------------------

func TestToolUse(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "tool_use", "id": "toolu_1", "name": "read", "input": map[string]any{"path": "main.go"}},
			},
			"usage": map[string]int{"input_tokens": 20, "output_tokens": 30},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "read main.go"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the request body had tools[] and tool_choice.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	// No tools were passed, but tool_choice should only be set if tools exist.
	if _, ok := reqBody["tool_choice"]; ok {
		t.Error("tool_choice should not be present when no tools are sent")
	}
}

func TestToolUseInStreamRequest(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_1\"}}\n\n"))
		w.Write([]byte("event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"echo\"}}\n\n"))
		w.Write([]byte("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"msg\\\":\\\"hello\\\"}\"}}\n\n"))
		w.Write([]byte("event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"))
		w.Write([]byte("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "echo hello"}},
		Tools: []core.ToolSpec{
			{Name: "echo", Description: "echoes input", Schema: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	hasToolStart, hasToolEnd, hasToolDelta := false, false, false
	for ev := range ch {
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

	// Verify tools were sent in request.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	toolsRaw, ok := reqBody["tools"]
	if !ok {
		t.Fatal("expected tools in request body")
	}
	tools := toolsRaw.([]any)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	tool := tools[0].(map[string]any)
	if tool["name"] != "echo" {
		t.Errorf("expected tool name 'echo', got %q", tool["name"])
	}
	tc, ok := reqBody["tool_choice"]
	if !ok {
		t.Error("expected tool_choice in request body")
	}
	tcMap := tc.(map[string]any)
	if tcMap["type"] != "auto" {
		t.Errorf("expected tool_choice.type = 'auto', got %q", tcMap["type"])
	}
}

// --- Requirement 5: Token usage (via Complete + message_delta event) ---------

func TestTokenUsageInComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "tok"}},
			"usage":   map[string]int{"input_tokens": 42, "output_tokens": 7},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != 42 {
		t.Errorf("expected 42 input tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 7 {
		t.Errorf("expected 7 output tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 49 {
		t.Errorf("expected 49 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

// --- Requirement 6: Error classification -------------------------------------

func TestError429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"Too many requests"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient error, got %v", err)
	}
}

func TestError400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"Bad request"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !core.IsKind(err, core.KindUsage) {
		t.Errorf("expected UsageError, got %v", err)
	}
}

func TestError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"type":"authentication_error","message":"Invalid API key"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !core.IsKind(err, core.KindPermanent) {
		t.Errorf("expected Permanent error, got %v", err)
	}
}

func TestError500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"type":"server_error","message":"Internal error"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient error, got %v", err)
	}
}

// --- Requirement 9: Empty response -------------------------------------------

func TestEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{},
			"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
}

// --- Requirement 10: Rate limit retry ----------------------------------------

func TestRateLimitRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"type":"rate_limit_error","message":"Rate limited"}}`))
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "success"}},
			"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	// Provider returns 429 as Transient — retry logic is in core/retry, not here.
	// We verify that the provider correctly classifies 429 as Transient.
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected rate limit error")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient, got %v", err)
	}
}

// --- System prompt + stop sequences (requirement 1 extras) -------------------

func TestSystemPromptAndStopSequences(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{{"type": "text", "text": "ok"}},
			"usage":   map[string]int{"input_tokens": 1, "output_tokens": 1},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv.URL, srv.Client())
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:        core.ModelSpec{Name: "claude-sonnet-4-6"},
		Messages:     []core.Message{{Role: core.RoleUser, Content: "hi"}},
		SystemPrompt: "You are helpful.",
		Options: core.ProviderOptions{
			Stop: []string{"END", "STOP"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	if reqBody["system"] != "You are helpful." {
		t.Errorf("expected system = 'You are helpful.', got %q", reqBody["system"])
	}
	stops := reqBody["stop_sequences"].([]any)
	if len(stops) != 2 {
		t.Errorf("expected 2 stop sequences, got %d", len(stops))
	}
}

// --- Error classification table ----------------------------------------------

func TestErrorClassificationTable(t *testing.T) {
	tests := []struct {
		status  int
		body    string
		wantErr string
	}{
		{429, `{"error":{"type":"rate_limit_error","message":"Too many requests"}}`, "rate limited"},
		{500, `{"error":{"type":"server_error","message":"Internal"}}`, "server error"},
		{503, `{"error":{"type":"overloaded","message":"Overloaded"}}`, "server error"},
		{401, `{"error":{"type":"authentication_error","message":"Invalid key"}}`, "auth error"},
		{403, `{"error":{"type":"permission_error","message":"Forbidden"}}`, "auth error"},
		{400, `{"error":{"type":"invalid_request_error","message":"Bad request"}}`, "bad request"},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
			w.Write([]byte(tt.body))
		}))

		p := newTestProvider(srv.URL, srv.Client())
		_, err := p.Stream(context.Background(), core.StreamRequest{
			Model: core.ModelSpec{Name: "claude-sonnet-4-6"},
		})

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
