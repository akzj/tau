package google_genai

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

	prov, err := NewProvider()
	if err != nil {
		t.Skip("no API key")
	}
	var _ core.Provider = prov
}

func TestErrorOnEmptyAPIKey(t *testing.T) {
	os.Unsetenv("ANTHROPIC_AUTH_TOKEN")
	_, err := NewProvider()
	if err == nil {
		t.Error("expected error for missing API key")
	}
}

func TestGeminiSSETextDelta(t *testing.T) {
	sseData := `data: {"candidates":[{"content":{"role":"model","parts":[{"text":"hello"}]},"finishReason":"STOP"}]}
data: {"candidates":[{"content":{"role":"model","parts":[{"text":" from gemini"}]},"finishReason":"STOP"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, ":streamGenerateContent") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gemini-2.5-flash", API: core.WireGoogleGenerativeAI},
	})
	if err != nil {
		t.Fatal(err)
	}

	var deltas string
	msgStart := false
	msgEnd := false
	for ev := range events {
		switch ev.Type {
		case core.ProvMessageStart:
			msgStart = true
		case core.ProvContentDelta:
			deltas += ev.ContentDelta
		case core.ProvMessageEnd:
			msgEnd = true
		case core.ProvError:
			t.Errorf("unexpected error: %v", ev.Err)
		}
	}
	if !msgStart {
		t.Error("missing ProvMessageStart")
	}
	if !msgEnd {
		t.Error("missing ProvMessageEnd")
	}
	if deltas != "hello from gemini" {
		t.Errorf("expected 'hello from gemini', got %q", deltas)
	}
}

func TestGeminiSSEFunctionCall(t *testing.T) {
	sseData := `data: {"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"get_weather","args":{"location":"NYC"}}}]},"finishReason":"STOP"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte(sseData))
	}))
	defer srv.Close()

	prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
	events, err := prov.Stream(context.Background(), core.StreamRequest{
		Model: core.ModelSpec{Name: "gemini-2.5-flash", API: core.WireGoogleGenerativeAI},
	})
	if err != nil {
		t.Fatal(err)
	}

	var toolStart, toolDelta, toolEnd bool
	for ev := range events {
		switch ev.Type {
		case core.ProvToolCallStart:
			toolStart = true
			if ev.ToolName != "get_weather" {
				t.Errorf("expected tool name 'get_weather', got %q", ev.ToolName)
			}
		case core.ProvToolCallDelta:
			toolDelta = true
		case core.ProvToolCallEnd:
			toolEnd = true
		}
	}
	if !toolStart {
		t.Error("missing ProvToolCallStart")
	}
	if !toolDelta {
		t.Error("missing ProvToolCallDelta")
	}
	if !toolEnd {
		t.Error("missing ProvToolCallEnd")
	}
}

func TestGeminiErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		expectMsg string
	}{
		{"rate_limited", 429, "rate limited (429)"},
		{"auth_error_401", 401, "auth error (401)"},
		{"auth_error_403", 403, "auth error (403)"},
		{"server_error", 500, "server error (500)"},
		{"server_error_502", 502, "server error (502)"},
		{"client_error", 400, "API error 400"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"error":{"message":"test error"}}`))
			}))
			defer srv.Close()

			prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
			_, err := prov.Stream(context.Background(), core.StreamRequest{
				Model: core.ModelSpec{Name: "gemini-2.5-flash", API: core.WireGoogleGenerativeAI},
			})
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectMsg) {
				t.Errorf("expected error containing %q, got %q", tt.expectMsg, err.Error())
			}
		})
	}
}

func TestGeminiCompleteErrorClassification(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		expectMsg string
	}{
		{"rate_limited", 429, "rate limited (429)"},
		{"auth_error", 401, "auth error (401)"},
		{"server_error", 500, "server error (500)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(`{"error":{"message":"test error"}}`))
			}))
			defer srv.Close()

			prov := &Provider{baseURL: srv.URL, apiKey: "test", client: &http.Client{}}
			_, err := prov.Complete(context.Background(), core.CompleteRequest{
				Model: core.ModelSpec{Name: "gemini-2.5-flash", API: core.WireGoogleGenerativeAI},
			})
			if err == nil {
				t.Error("expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.expectMsg) {
				t.Errorf("expected error containing %q, got %q", tt.expectMsg, err.Error())
			}
		})
	}
}

func TestBuildContentsToolResponse(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleAssistant, ToolCalls: []core.ToolCallRequest{{CallID: "call_1", ToolName: "echo", Args: `{"msg":"hi"}`}}},
		{Role: core.RoleTool, ToolCallID: "call_1", Content: "echo: hi"},
	}
	contents := buildContents(msgs)
	if len(contents) < 2 {
		t.Fatalf("expected at least 2 contents, got %d", len(contents))
	}

	// The tool response should have functionResponse with the tool name, not call ID.
	toolMsg := contents[1]
	if toolMsg["role"] != "function" {
		t.Errorf("expected role 'function', got %q", toolMsg["role"])
	}
	parts := toolMsg["parts"].([]map[string]any)
	fr := parts[0]["functionResponse"].(map[string]any)
	if fr["name"] != "echo" {
		t.Errorf("expected functionResponse.name='echo', got %q (should be tool name, not call ID)", fr["name"])
	}
}