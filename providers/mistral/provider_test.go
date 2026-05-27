package mistral

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
)

// newTestProvider creates a provider backed by an httptest server (test helper).
func newTestProvider(srv *httptest.Server) *Provider {
	return &Provider{
		baseURL:   srv.URL,
		apiKey:    "test-key",
		client:    &http.Client{},
		maxTokens: 4096,
	}
}

// TestNewProviderEnv tests environment variable handling.
func TestNewProviderEnv(t *testing.T) {
	os.Unsetenv("MISTRAL_API_KEY")

	_, err := NewProvider()
	if err == nil {
		t.Error("expected error for missing MISTRAL_API_KEY")
	}

	os.Setenv("MISTRAL_API_KEY", "test-key")
	prov, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.baseURL != defaultMistralBaseURL {
		t.Errorf("expected baseURL %s, got %q", defaultMistralBaseURL, prov.baseURL)
	}
}

// TestCompleteText tests full text completion with token usage.
func TestCompleteText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify headers.
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Authorization Bearer header, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "hello world"}, "finish_reason": "stop"},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 2,
				"total_tokens":      12,
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:        core.ModelSpec{Name: "gpt-4"},
		Messages:     []core.Message{{Role: core.RoleUser, Content: "hi"}},
		SystemPrompt: "You are helpful.",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Errorf("expected 12 tokens, got %d", resp.Usage.TotalTokens)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("expected 10 prompt tokens, got %d", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 2 {
		t.Errorf("expected 2 completion tokens, got %d", resp.Usage.CompletionTokens)
	}
}

// TestStreamText tests streaming SSE parsing with multiple content deltas.
func TestStreamText(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-001","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"hello "}}]}
data: {"id":"chatcmpl-001","choices":[{"index":0,"delta":{"content":"world"}}]}
data: {"id":"chatcmpl-001","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}
data: [DONE]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "gpt-4"}})
	if err != nil {
		t.Fatal(err)
	}

	var (
		gotStart bool
		gotEnd   bool
		gotUsage bool
		content  string
	)
	for ev := range ch {
		switch ev.Type {
		case core.ProvMessageStart:
			gotStart = true
			if ev.MessageID != "chatcmpl-001" {
				t.Errorf("expected messageID chatcmpl-001, got %q", ev.MessageID)
			}
		case core.ProvContentDelta:
			content += ev.ContentDelta
		case core.ProvUsage:
			gotUsage = true
			if ev.Usage.TotalTokens != 12 {
				t.Errorf("expected 12 tokens, got %d", ev.Usage.TotalTokens)
			}
		case core.ProvMessageEnd:
			gotEnd = true
		case core.ProvError:
			t.Errorf("unexpected error: %v", ev.Err)
		}
	}

	if !gotStart {
		t.Error("expected ProvMessageStart")
	}
	if !gotEnd {
		t.Error("expected ProvMessageEnd")
	}
	if !gotUsage {
		t.Error("expected ProvUsage")
	}
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

// TestVisionImage tests image content blocks in messages.
func TestVisionImage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		messages, ok := body["messages"].([]any)
		if !ok || len(messages) == 0 {
			t.Error("expected messages array")
		}

		// Last message should have image_url content.
		lastMsg := messages[len(messages)-1].(map[string]any)
		content, ok := lastMsg["content"].([]any)
		if !ok {
			t.Errorf("expected array content for vision message, got %T", lastMsg["content"])
			return
		}

		foundImage := false
		for _, c := range content {
			block := c.(map[string]any)
			if block["type"] == "image_url" {
				foundImage = true
				imgURL := block["image_url"].(map[string]any)
				if !strings.Contains(imgURL["url"].(string), "data:image/png;base64,") {
					t.Errorf("expected base64 data URI, got %q", imgURL["url"])
				}
			}
		}
		if !foundImage {
			t.Error("expected image_url block")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "I see an image."}},
			},
			"usage": map[string]any{
				"prompt_tokens": 100, "completion_tokens": 5, "total_tokens": 105,
			},
		})
	}))
	defer srv.Close()

	// Build Anthropic-style vision content block.
	visionJSON := `[
		{"type":"text","text":"Describe this image:"},
		{"type":"image","source":{"type":"base64","media_type":"image/png","data":"iVBORw0KGgo="}}
	]`

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4-vision"},
		Messages: []core.Message{{Role: core.RoleUser, Content: visionJSON}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "I see an image." {
		t.Errorf("expected 'I see an image.', got %q", resp.Content)
	}
}

// TestFunctionCalling tests tool_use in stream request.
func TestFunctionCalling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		// Verify tools are sent.
		tools, ok := body["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Error("expected tools array in request")
		}
		if body["tool_choice"] != "auto" {
			t.Error("expected tool_choice: auto")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"id":"chatcmpl-003","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call-abc123","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"id":"chatcmpl-003","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":\"Paris\"}"}}]}}]}
data: {"id":"chatcmpl-003","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":30,"completion_tokens":15,"total_tokens":45}}
data: [DONE]`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-4"},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather in Paris?"},
		},
		Tools: []core.ToolSpec{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Schema:      map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Drain the channel; verify tools were sent (checked in handler above).
	gotUsage := false
	for ev := range ch {
		if ev.Type == core.ProvUsage && ev.Usage != nil {
			gotUsage = true
			if ev.Usage.TotalTokens != 45 {
				t.Errorf("expected 45 tokens, got %d", ev.Usage.TotalTokens)
			}
		}
		if ev.Type == core.ProvError {
			t.Errorf("unexpected error: %v", ev.Err)
		}
	}
	if !gotUsage {
		t.Error("expected ProvUsage event")
	}
}

// TestFunctionCallingInStream tests tool calls during streaming.
func TestFunctionCallingInStream(t *testing.T) {
	sseData := `data: {"id":"chatcmpl-002","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call-xyz","type":"function","function":{"name":"get_weather","arguments":""}}]}}]}
data: {"id":"chatcmpl-002","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"city\":"}}]}}]}
data: {"id":"chatcmpl-002","choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"Paris\"}"}}]}}]}
data: {"id":"chatcmpl-002","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}
data: [DONE]`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-4"},
		Tools: []core.ToolSpec{
			{
				Name:        "get_weather",
				Description: "Get weather",
				Schema:      map[string]any{"type": "object"},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var (
		gotToolStart bool
		gotToolDelta bool
		gotToolEnd   bool
		toolName     string
		args         string
	)
	for ev := range ch {
		switch ev.Type {
		case core.ProvToolCallStart:
			gotToolStart = true
			toolName = ev.ToolName
			if ev.ToolCallID != "call-xyz" {
				t.Errorf("expected toolCallID call-xyz, got %q", ev.ToolCallID)
			}
		case core.ProvToolCallDelta:
			gotToolDelta = true
			args += ev.ToolArgsDelta
		case core.ProvToolCallEnd:
			gotToolEnd = true
		case core.ProvError:
			t.Errorf("unexpected error: %v", ev.Err)
		}
	}

	if !gotToolStart {
		t.Error("expected ProvToolCallStart")
	}
	if !gotToolDelta {
		t.Error("expected ProvToolCallDelta")
	}
	if !gotToolEnd {
		t.Error("expected ProvToolCallEnd")
	}
	if toolName != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", toolName)
	}
	if args != `{"city":"Paris"}` {
		t.Errorf("expected args '{\"city\":\"Paris\"}', got %q", args)
	}
}

// TestErrorClassification tests error classification via table-driven approach.
func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		body            string
		expectTransient bool
		expectPerm      bool
		expectUsage     bool
	}{
		{name: "429 rate limited", statusCode: 429, body: `{"error":{"message":"rate limit"}}`, expectTransient: true},
		{name: "400 bad request", statusCode: 400, body: `{"error":{"message":"invalid"}}`, expectUsage: true},
		{name: "401 unauthorized", statusCode: 401, body: `{"error":{"message":"unauthorized"}}`, expectPerm: true},
		{name: "403 forbidden", statusCode: 403, body: `{"error":{"message":"forbidden"}}`, expectPerm: true},
		{name: "500 internal", statusCode: 500, body: `{"error":{"message":"server error"}}`, expectTransient: true},
		{name: "502 bad gateway", statusCode: 502, body: `{"error":{"message":"bad gateway"}}`, expectTransient: true},
		{name: "503 unavailable", statusCode: 503, body: `{"error":{"message":"unavailable"}}`, expectTransient: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			prov := newTestProvider(srv)
			_, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
			if err == nil {
				t.Fatal("expected error")
			}

			if tt.expectTransient && !core.IsKind(err, core.KindTransient) {
				t.Errorf("expected Transient, got kind=%v: %v", core.KindOf(err), err)
			}
			if tt.expectPerm && !core.IsKind(err, core.KindPermanent) {
				t.Errorf("expected Permanent, got kind=%v: %v", core.KindOf(err), err)
			}
			if tt.expectUsage && !core.IsKind(err, core.KindUsage) {
				t.Errorf("expected Usage, got kind=%v: %v", core.KindOf(err), err)
			}
		})
	}
}

// TestEmptyResponse tests handling of empty choices.
func TestEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{},
			"usage": map[string]any{
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"total_tokens":      0,
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 0 {
		t.Errorf("expected 0 tokens, got %d", resp.Usage.TotalTokens)
	}
}

// TestContextCancel tests cancelled context propagation in Complete.
func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"content": "ok"}},
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := prov.Complete(ctx, core.CompleteRequest{
		Model:    core.ModelSpec{Name: "gpt-4"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// TestStreamOnPayload tests the OnPayload transform hook.
func TestStreamOnPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"id":"x","choices":[{"index":0,"delta":{"content":"ok"}}]}
data: [DONE]`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	called := false
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-4"},
		OnPayload: func(body any) (any, error) {
			called = true
			return body, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if !called {
		t.Error("expected OnPayload to be called")
	}
}

// TestStreamOnResponse tests the OnResponse hook.
func TestStreamOnResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Custom", "test-value")
		w.Write([]byte(`data: {"id":"x","choices":[{"index":0,"delta":{"content":"ok"}}]}
data: [DONE]`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	var gotHeaders map[string][]string
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gpt-4"},
		OnResponse: func(statusCode int, headers map[string][]string) {
			gotHeaders = headers
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if gotHeaders["X-Custom"][0] != "test-value" {
		t.Error("expected X-Custom header")
	}
}

// TestCountTokens tests the token estimation.
func TestCountTokens(t *testing.T) {
	prov := New("test-key")
	if n := prov.CountTokens("hello world"); n != 2 {
		t.Errorf("expected ~2 tokens for 'hello world', got %d", n)
	}
	if n := prov.CountTokens(""); n != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", n)
	}
}

// TestCompat tests the Compat() method.
func TestCompat(t *testing.T) {
	prov := New("test-key")
	compat := prov.Compat()
	oc, ok := compat.(core.OpenAICompletionsCompat)
	if !ok {
		t.Fatalf("expected core.OpenAICompletionsCompat, got %T", compat)
	}
	if !oc.TemperatureField {
		t.Error("expected TemperatureField=true")
	}
	if !oc.SupportsStop {
		t.Error("expected SupportsStop=true")
	}
	if !oc.MaxTokensField {
		t.Error("expected MaxTokensField=true")
	}
}

// TestConvertMessagesSystemPrompt tests system prompt placement.
func TestConvertMessagesSystemPrompt(t *testing.T) {
	msgs := convertMessages([]core.Message{
		{Role: core.RoleUser, Content: "hello"},
	}, "You are a helpful assistant.")

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0]["role"] != "system" {
		t.Errorf("expected system role, got %v", msgs[0]["role"])
	}
	if msgs[0]["content"] != "You are a helpful assistant." {
		t.Errorf("unexpected system content: %v", msgs[0]["content"])
	}
}

// TestConvertMessagesToolCalls tests assistant tool_calls conversion.
func TestConvertMessagesToolCalls(t *testing.T) {
	msgs := convertMessages([]core.Message{
		{Role: core.RoleAssistant, ToolCalls: []core.ToolCallRequest{
			{CallID: "call-1", ToolName: "echo", Args: `{"msg":"hi"}`},
		}},
	}, "")

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	tcs, ok := msgs[0]["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatal("expected tool_calls array")
	}
	if len(tcs) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(tcs))
	}
	if tcs[0]["id"] != "call-1" {
		t.Errorf("expected call-1, got %v", tcs[0]["id"])
	}
}

// TestConvertMessagesToolResult tests tool result message conversion.
func TestConvertMessagesToolResult(t *testing.T) {
	msgs := convertMessages([]core.Message{
		{Role: core.RoleTool, ToolCallID: "call-xyz", Content: `{"result":"ok"}`},
	}, "")

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0]["role"] != "tool" {
		t.Errorf("expected role tool, got %v", msgs[0]["role"])
	}
	if msgs[0]["tool_call_id"] != "call-xyz" {
		t.Errorf("expected tool_call_id call-xyz, got %v", msgs[0]["tool_call_id"])
	}
}

// TestConvertTools tests tool spec conversion.
func TestConvertTools(t *testing.T) {
	tools := convertTools([]core.ToolSpec{
		{Name: "my_func", Description: "Does something", Schema: map[string]any{"type": "object"}},
	}, nil)

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["type"] != "function" {
		t.Errorf("expected type function, got %v", tools[0]["type"])
	}
	fn := tools[0]["function"].(map[string]any)
	if fn["name"] != "my_func" {
		t.Errorf("expected name my_func, got %v", fn["name"])
	}
}

// TestConvertToolsTransform tests tool name transformation.
func TestConvertToolsTransform(t *testing.T) {
	tools := convertTools([]core.ToolSpec{
		{Name: "my_func", Description: "Does something", Schema: map[string]any{"type": "object"}},
	}, func(s string) string { return "prefix_" + s })

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	fn := tools[0]["function"].(map[string]any)
	if fn["name"] != "prefix_my_func" {
		t.Errorf("expected name prefix_my_func, got %v", fn["name"])
	}
}