package tools_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestWebScrapeTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><div class="content">Target</div><p>Other</p></body></html>`))
	}))
	defer srv.Close()

	tool := tools.WebScrapeTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "selector": "div.content"}, nil)
	if err != nil {
		t.Fatalf("web_scrape: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Target") {
		t.Errorf("expected Target, got: %s", result.Content[0].Text)
	}
}

func TestWebScrapeNoMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html><body><p>Only</p></body></html>`))
	}))
	defer srv.Close()

	tool := tools.WebScrapeTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"url": srv.URL, "selector": "div.nonexistent"}, nil)
	if !strings.Contains(result.Content[0].Text, "No elements") {
		t.Errorf("expected no-match message, got: %s", result.Content[0].Text)
	}
}
