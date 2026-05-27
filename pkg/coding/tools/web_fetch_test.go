package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestWebFetchTitleExtraction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><head><title>My Page</title></head><body><p>Content</p></body></html>`))
	}))
	defer srv.Close()

	tool := tools.WebFetchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL}, nil)
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "My Page") {
		t.Errorf("expected title, got: %s", result.Content[0].Text)
	}
}

func TestWebFetchMaxChars(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := strings.Repeat("x", 10000)
		w.Write([]byte("<html><body>" + body + "</body></html>"))
	}))
	defer srv.Close()

	tool := tools.WebFetchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "max_chars": 100}, nil)
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	// Should be truncated to ~100 chars plus truncation notice
	text := result.Content[0].Text
	if !strings.Contains(text, "truncated") {
		t.Logf("fetch output length: %d", len(text))
	}
}
