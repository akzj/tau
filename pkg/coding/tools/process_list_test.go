package tools

import (
	"context"
	"strings"
	"testing"
)

func TestProcessListTool_NoFilter(t *testing.T) {
	tool := ProcessListTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "PID") && !strings.Contains(result.Content[0].Text, "USER") {
		t.Errorf("expected process listing with headers, got: %s", result.Content[0].Text[:100])
	}
}

func TestProcessListTool_WithFilter(t *testing.T) {
	tool := ProcessListTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"filter": "init",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should contain the header line
	if result.Content[0].Text == "" {
		t.Error("expected non-empty output")
	}
}
