package cohere

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

// --- Unit: convertMessages ---

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

// --- Unit: convertTools ---

func TestConvertTools(t *testing.T) {
	tools := convertTools([]core.ToolSpec{
		{
			Name:        "my_func",
			Description: "Does something",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"msg": map[string]any{"type": "string", "description": "a message"},
				},
			},
		},
	}, nil)

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["name"] != "my_func" {
		t.Errorf("expected name my_func, got %v", tools[0]["name"])
	}
	if tools[0]["description"] != "Does something" {
		t.Errorf("expected description 'Does something', got %v", tools[0]["description"])
	}

	pd, ok := tools[0]["parameter_definitions"].(map[string]any)
	if !ok {
		t.Fatal("expected parameter_definitions")
	}
	if pd["msg"] == nil {
		t.Error("expected msg in parameter_definitions")
	}
}

func TestConvertToolsTransform(t *testing.T) {
	tools := convertTools([]core.ToolSpec{
		{Name: "my_func", Description: "Desc", Schema: map[string]any{"type": "object"}},
	}, func(s string) string { return "prefix_" + s })

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0]["name"] != "prefix_my_func" {
		t.Errorf("expected name prefix_my_func, got %v", tools[0]["name"])
	}
}

// --- Complete tests ---

func TestCompleteText(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header.
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "hello world"},
				},
			},
			"usage": map[string]any{
				"billed_units": map[string]any{
					"input_tokens":  10,
					"output_tokens": 2,
				},
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:        core.ModelSpec{Name: "command-r"},
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

func TestCompleteMultiBlock(t *testing.T) {
	// Multiple content blocks — text should be concatenated.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"content": []map[string]any{
					{"type": "text", "text": "part one "},
					{"type": "text", "text": "part two"},
				},
			},
			"usage": map[string]any{
				"billed_units": map[string]any{
					"input_tokens":  5,
					"output_tokens": 10,
				},
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "command-r"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "part one part two" {
		t.Errorf("expected 'part one part two', got %q", resp.Content)
	}
}

func TestCompleteToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"content": []map[string]any{},
				"tool_calls": []map[string]any{
					{
						"id":   "call-abc",
						"type": "function",
						"function": map[string]any{
							"name":      "get_weather",
							"arguments": `{"city":"Paris"}`,
						},
					},
				},
			},
			"usage": map[string]any{
				"billed_units": map[string]any{
					"input_tokens":  15,
					"output_tokens": 8,
				},
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "command-r"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "Weather in Paris?"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Content may be empty when only tool calls are present.
	if resp.Usage.TotalTokens != 23 {
		t.Errorf("expected 23 tokens, got %d", resp.Usage.TotalTokens)
	}
}

// --- Stream tests ---

func TestStreamText(t *testing.T) {
	sseData := `data: {"type":"content-start","index":0}
data: {"type":"content-delta","delta":{"message":{"content":{"text":"hello "}}}}
data: {"type":"content-delta","delta":{"message":{"content":{"text":"world"}}}}
data: {"type":"content-end","index":0,"finish_reason":"COMPLETE"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "command-r"}})
	if err != nil {
		t.Fatal(err)
	}

	var (
		gotStart bool
		gotEnd   bool
		content  string
	)
	for ev := range ch {
		switch ev.Type {
		case core.ProvMessageStart:
			gotStart = true
		case core.ProvContentDelta:
			content += ev.ContentDelta
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
	if content != "hello world" {
		t.Errorf("expected 'hello world', got %q", content)
	}
}

func TestStreamToolCall(t *testing.T) {
	sseData := `data: {"type":"content-start","index":0}
data: {"type":"tool-call-start","delta":{"message":{"tool_calls":{"id":"call-xyz","type":"function","function":{"name":"get_weather"}}}}}
data: {"type":"tool-call-delta","delta":{"message":{"tool_calls":{"function":{"arguments":"{\"city\":\""}}}}}
data: {"type":"tool-call-delta","delta":{"message":{"tool_calls":{"function":{"arguments":"Paris\"}"}}}}}
data: {"type":"tool-call-end","delta":{"message":{"tool_calls":{"id":"call-xyz"}}}}
data: {"type":"content-end","finish_reason":"COMPLETE"}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "command-r"},
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

// --- Error classification ---

func TestErrorClassification(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		body            string
		expectTransient bool
		expectPerm      bool
		expectUsage     bool
	}{
		{name: "429 rate limited", statusCode: 429, body: `{"message":"rate limit"}`, expectTransient: true},
		{name: "400 bad request", statusCode: 400, body: `{"message":"invalid"}`, expectUsage: true},
		{name: "401 unauthorized", statusCode: 401, body: `{"message":"unauthorized"}`, expectPerm: true},
		{name: "403 forbidden", statusCode: 403, body: `{"message":"forbidden"}`, expectPerm: true},
		{name: "500 internal", statusCode: 500, body: `{"message":"server error"}`, expectTransient: true},
		{name: "503 unavailable", statusCode: 503, body: `{"message":"unavailable"}`, expectTransient: true},
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

// --- Edge cases ---

func TestEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{
				"content": []map[string]any{},
			},
			"usage": map[string]any{
				"billed_units": map[string]any{
					"input_tokens":  5,
					"output_tokens": 0,
				},
			},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	resp, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "command-r"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "" {
		t.Errorf("expected empty content, got %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Errorf("expected 5 tokens, got %d", resp.Usage.TotalTokens)
	}
}

func TestContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"message": map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}},
			"usage":   map[string]any{"billed_units": map[string]any{"input_tokens": 1, "output_tokens": 1}},
		})
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := prov.Complete(ctx, core.CompleteRequest{
		Model:    core.ModelSpec{Name: "command-r"},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestCountTokens(t *testing.T) {
	prov := New("test-key")
	if n := prov.CountTokens("hello world"); n != 2 {
		t.Errorf("expected ~2 tokens for 'hello world', got %d", n)
	}
	if n := prov.CountTokens(""); n != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", n)
	}
}

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

// --- Hooks ---

func TestStreamOnPayload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"type":"content-start","index":0}
data: {"type":"content-delta","delta":{"message":{"content":{"text":"ok"}}}}
data: {"type":"content-end","finish_reason":"COMPLETE"}`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	called := false
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "command-r"},
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

func TestStreamOnResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Custom", "test-value")
		w.Write([]byte(`data: {"type":"content-start","index":0}
data: {"type":"content-delta","delta":{"message":{"content":{"text":"ok"}}}}
data: {"type":"content-end","finish_reason":"COMPLETE"}`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	var gotHeaders map[string][]string
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "command-r"},
		OnResponse: func(statusCode int, headers map[string][]string) {
			gotHeaders = headers
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
	if gotHeaders == nil {
		t.Fatal("expected OnResponse to be called")
	}
	if gotHeaders["X-Custom"][0] != "test-value" {
		t.Error("expected X-Custom header")
	}
}

// --- NewProvider env ---

func TestNewProviderEnv(t *testing.T) {
	os.Unsetenv("COHERE_API_KEY")
	os.Unsetenv("COHERE_BASE_URL")

	_, err := NewProvider()
	if err == nil {
		t.Error("expected error for missing COHERE_API_KEY")
	}
	if !strings.Contains(err.Error(), "COHERE_API_KEY") {
		t.Errorf("expected error mentioning COHERE_API_KEY, got %v", err)
	}

	os.Setenv("COHERE_API_KEY", "test-key")
	prov, err := NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.baseURL != "https://api.cohere.com/v2" {
		t.Errorf("expected default baseURL, got %q", prov.baseURL)
	}
	if prov.apiKey != "test-key" {
		t.Errorf("expected apiKey 'test-key', got %q", prov.apiKey)
	}

	// Custom base URL.
	os.Setenv("COHERE_BASE_URL", "https://custom.cohere.com/v2/")
	prov, err = NewProvider()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prov.baseURL != "https://custom.cohere.com/v2" {
		t.Errorf("expected trimmed baseURL, got %q", prov.baseURL)
	}
}

// --- Tool request verification ---

func TestStreamToolsInRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		tools, ok := body["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Error("expected tools array in request")
		}

		// Verify Cohere format: parameter_definitions, not parameters.
		if len(tools) > 0 {
			tool := tools[0].(map[string]any)
			if _, ok := tool["parameter_definitions"]; !ok {
				t.Error("expected parameter_definitions in tool")
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(`data: {"type":"content-start","index":0}
data: {"type":"content-delta","delta":{"message":{"content":{"text":"done"}}}}
data: {"type":"content-end","finish_reason":"COMPLETE"}`))
	}))
	defer srv.Close()

	prov := newTestProvider(srv)
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "command-r"},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "What's the weather?"},
		},
		Tools: []core.ToolSpec{
			{
				Name:        "get_weather",
				Description: "Get weather for a city",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{"type": "string"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for range ch {
	}
}
