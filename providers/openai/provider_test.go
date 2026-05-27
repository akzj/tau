//go:build !no_openai

package openai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

// --- helpers ----------------------------------------------------------------

func newTestProvider(srv *httptest.Server) *Provider {
	return &Provider{
		baseURL:   srv.URL,
		apiKey:    "test-key",
		client:    srv.Client(),
		maxTokens: 100,
	}
}

// --- Requirement 1: Text completion -----------------------------------------

func TestCompleteText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "hello world"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
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

// --- Requirement 2: Streaming -----------------------------------------------

func TestStreamText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"hello\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
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

// --- Requirement 3: Vision (image content blocks) ---------------------------

func TestVisionImage(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "image analyzed"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     100,
				"completion_tokens": 10,
				"total_tokens":      110,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	// Anthropic-style vision content blocks — should be converted to OpenAI image_url format.
	visionBlocks := `[{"type":"image","source":{"type":"base64","media_type":"image/png","data":"base64data"}},{"type":"text","text":"describe this image"}]`
	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: visionBlocks}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "image analyzed" {
		t.Errorf("expected 'image analyzed', got %q", resp.Content)
	}

	// Verify the request body has proper OpenAI image_url content blocks.
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	msgs := reqBody["messages"].([]any)
	msg := msgs[0].(map[string]any)
	content := msg["content"].([]any)

	// First block should be the image (converted to image_url).
	img := content[0].(map[string]any)
	if img["type"] != "image_url" {
		t.Errorf("expected content[0].type = 'image_url', got %q", img["type"])
	}
	imageURL := img["image_url"].(map[string]any)
	if url, ok := imageURL["url"].(string); ok {
		if !strings.Contains(url, "data:image/png;base64,base64data") {
			t.Errorf("expected data URI, got %q", url)
		}
	} else {
		t.Error("expected image_url.url to be a string")
	}

	// Second block should be the text.
	txt := content[1].(map[string]any)
	if txt["type"] != "text" {
		t.Errorf("expected content[1].type = 'text', got %q", txt["type"])
	}
}

// --- Requirement 4: Function calling (tools in request) ---------------------

func TestFunctionCalling(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role": "assistant",
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "get_weather",
									"arguments": `{"location":"NYC"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     20,
				"completion_tokens": 30,
				"total_tokens":      50,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "what's the weather in NYC?"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify tools were NOT sent (Complete doesn't pass Tools).
	var reqBody map[string]any
	if err := json.Unmarshal(capturedBody, &reqBody); err != nil {
		t.Fatal(err)
	}
	if _, ok := reqBody["tool_choice"]; ok {
		t.Error("tool_choice should not be present when no tools are sent")
	}
}

func TestFunctionCallingInStreamRequest(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\",\"type\":\"function\",\"function\":{\"name\":\"echo\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"msg\\\":\\\"hello\\\"}\"}}]},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "echo hello"}},
		Tools: []core.ToolSpec{
			{
				Name:        "echo",
				Description: "echoes input",
				Schema:      json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	hasToolStart, hasToolDelta, hasToolEnd := false, false, false
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

	// Verify tools were sent in request with proper OpenAI format.
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
	if tool["type"] != "function" {
		t.Errorf("expected tool type 'function', got %q", tool["type"])
	}
	fn, ok := tool["function"].(map[string]any)
	if !ok {
		t.Fatal("expected tool.function")
	}
	if fn["name"] != "echo" {
		t.Errorf("expected tool name 'echo', got %q", fn["name"])
	}
	tc, ok := reqBody["tool_choice"]
	if !ok {
		t.Error("expected tool_choice in request body")
	}
	if tc != "auto" {
		t.Errorf("expected tool_choice = 'auto', got %q", tc)
	}
}

// --- Requirement 5: Streaming tool calls (SSE tool call parsing) ------------

func TestStreamingToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// First chunk: tool call start (ID + name + empty args).
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_abc\",\"type\":\"function\",\"function\":{\"name\":\"read\",\"arguments\":\"\"}}]},\"finish_reason\":null}]}\n\n"))
		// Second chunk: args fragment 1.
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"{\\\"path\\\":\"}}]},\"finish_reason\":null}]}\n\n"))
		// Third chunk: args fragment 2.
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0,\"function\":{\"arguments\":\"\\\"main.go\\\"}\"}}]},\"finish_reason\":null}]}\n\n"))
		// Finish: tool_calls.
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "read main.go"}},
		Tools: []core.ToolSpec{
			{
				Name:        "read",
				Description: "reads a file",
				Schema:      json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var startedCallID string
	var argsBuffer strings.Builder
	hasStart, hasEnd := false, false

	for ev := range ch {
		switch ev.Type {
		case core.ProvToolCallStart:
			hasStart = true
			startedCallID = ev.ToolCallID
			if ev.ToolName != "read" {
				t.Errorf("expected tool name 'read', got %q", ev.ToolName)
			}
		case core.ProvToolCallDelta:
			argsBuffer.WriteString(ev.ToolArgsDelta)
		case core.ProvToolCallEnd:
			hasEnd = true
			if ev.ToolCallID != startedCallID {
				t.Errorf("ToolCallEnd ID %q doesn't match start %q", ev.ToolCallID, startedCallID)
			}
		}
	}

	if !hasStart {
		t.Error("expected ProvToolCallStart event")
	}
	if !hasEnd {
		t.Error("expected ProvToolCallEnd event")
	}
	args := argsBuffer.String()
	if !strings.Contains(args, "main.go") {
		t.Errorf("expected args to contain 'main.go', got %q", args)
	}
}

// --- Requirement 6: Token usage (complete + streaming) ----------------------

func TestTokenUsageInComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "tok"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     42,
				"completion_tokens": 7,
				"total_tokens":      49,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Usage.PromptTokens != 42 {
		t.Errorf("expected 42 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 7 {
		t.Errorf("expected 7 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 49 {
		t.Errorf("expected 49 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestTokenUsageInStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi\"},\"finish_reason\":null}]}\n\n"))
		w.Write([]byte("data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":30,\"completion_tokens\":8,\"total_tokens\":38}}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	ch, err := p.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	hasUsage := false
	for ev := range ch {
		if ev.Type == core.ProvUsage {
			hasUsage = true
			if ev.Usage.PromptTokens != 30 {
				t.Errorf("expected 30 prompt tokens, got %d", ev.Usage.PromptTokens)
			}
			if ev.Usage.CompletionTokens != 8 {
				t.Errorf("expected 8 completion tokens, got %d", ev.Usage.CompletionTokens)
			}
			if ev.Usage.TotalTokens != 38 {
				t.Errorf("expected 38 total tokens, got %d", ev.Usage.TotalTokens)
			}
		}
	}
	if !hasUsage {
		t.Error("expected ProvUsage event in stream")
	}
}

// --- Requirement 7-10: Error classification ---------------------------------

func TestError429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		w.Write([]byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limit exceeded","code":"rate_limit_exceeded"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 429")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient error, got %T: %v", err, err)
	}
}

func TestError400(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":{"type":"invalid_request_error","message":"Bad request","code":"invalid_request_error"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !core.IsKind(err, core.KindUsage) {
		t.Errorf("expected Usage error, got %T: %v", err, err)
	}
}

func TestError401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":{"type":"authentication_error","message":"Invalid API key","code":"invalid_api_key"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !core.IsKind(err, core.KindPermanent) {
		t.Errorf("expected Permanent error, got %T: %v", err, err)
	}
}

func TestError500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		w.Write([]byte(`{"error":{"type":"server_error","message":"Internal error","code":"internal_error"}}`))
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient error, got %T: %v", err, err)
	}
}

// --- Requirement 11: Empty response -----------------------------------------

func TestEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	resp, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
}

// --- Requirement 12: Rate limit retry (classifies 429 as Transient) ---------

func TestRateLimitRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(429)
			w.Write([]byte(`{"error":{"type":"rate_limit_exceeded","message":"Rate limited","code":"rate_limit_exceeded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "success"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	// First calls will get 429 — verify provider correctly classifies as Transient.
	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4o"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected rate limit error")
	}
	if !core.IsKind(err, core.KindTransient) {
		t.Errorf("expected Transient, got %T: %v", err, err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (provider doesn't retry), got %d", attempts)
	}
}

// --- Error classification table ---------------------------------------------

func TestErrorClassificationTable(t *testing.T) {
	tests := []struct {
		status  int
		body    string
		wantErr string
	}{
		{429, `{"error":{"type":"rate_limit_exceeded","message":"Rate limited","code":"rate_limit_exceeded"}}`, "rate limited"},
		{500, `{"error":{"type":"server_error","message":"Internal error","code":"internal_error"}}`, "server error"},
		{503, `{"error":{"type":"server_error","message":"Overloaded","code":"server_error"}}`, "server error"},
		{401, `{"error":{"type":"authentication_error","message":"Invalid key","code":"invalid_api_key"}}`, "auth error"},
		{403, `{"error":{"type":"permission_error","message":"Forbidden","code":"permission_error"}}`, "auth error"},
		{400, `{"error":{"type":"invalid_request_error","message":"Bad request","code":"invalid_request_error"}}`, "bad request"},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
			w.Write([]byte(tt.body))
		}))

		p := newTestProvider(srv)
		_, err := p.Stream(context.Background(), core.StreamRequest{
			Model: core.ModelSpec{Name: "gpt-4o"},
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

// --- Backward compat --------------------------------------------------------

func TestCompat(t *testing.T) {
	p := New("test-key")
	c := p.Compat()

	oc, ok := c.(core.OpenAICompletionsCompat)
	if !ok {
		t.Fatalf("expected core.OpenAICompletionsCompat, got %T", c)
	}
	if !oc.TemperatureField {
		t.Error("expected TemperatureField = true")
	}
	if !oc.TopPField {
		t.Error("expected TopPField = true")
	}
	if !oc.SupportsStop {
		t.Error("expected SupportsStop = true")
	}
}

// --- CountTokens ------------------------------------------------------------

func TestCountTokens(t *testing.T) {
	p := New("test-key")
	n := p.CountTokens("hello world")
	if n != 2 { // 11 chars / 4 = 2
		t.Errorf("expected 2 tokens for 'hello world', got %d", n)
	}
}

// --- System prompt ----------------------------------------------------------

func TestSystemPromptAndStop(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"index":         0,
					"message":       map[string]any{"role": "assistant", "content": "ok"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]int{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		})
	}))
	defer srv.Close()
	p := newTestProvider(srv)

	_, err := p.Complete(context.Background(), core.CompleteRequest{
		Model:        core.ModelSpec{Name: "gpt-4o"},
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

	// Verify system prompt is first message.
	msgs := reqBody["messages"].([]any)
	sysMsg := msgs[0].(map[string]any)
	if sysMsg["role"] != "system" {
		t.Errorf("expected first message role 'system', got %q", sysMsg["role"])
	}
	if sysMsg["content"] != "You are helpful." {
		t.Errorf("expected system content 'You are helpful.', got %q", sysMsg["content"])
	}

	// Verify stop sequences.
	stops := reqBody["stop"].([]any)
	if len(stops) != 2 {
		t.Errorf("expected 2 stop sequences, got %d", len(stops))
	}
}
