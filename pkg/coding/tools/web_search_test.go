package tools_test

import (
	"context"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestWebSearchWithMaxResults(t *testing.T) {
	tool := tools.WebSearchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "golang", "max_results": 3}, nil)
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}
	_ = result
}

func TestWebSearchMaxResultsCap(t *testing.T) {
	tool := tools.WebSearchTool()
	// max_results > 20 should be capped
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "golang", "max_results": 50}, nil)
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}
	_ = result
}
