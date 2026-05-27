package tools_test

import (
	"context"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestTemplateRenderBasic(t *testing.T) {
	tool := tools.TemplateRenderTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"template": "Hello, {{.Name}}!",
		"vars":     `{"Name":"World"}`,
		"format":   "text",
	}, nil)
	if err != nil {
		t.Fatalf("template_render: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Hello, World!") {
		t.Errorf("expected 'Hello, World!', got: %s", result.Content[0].Text)
	}
}

func TestTemplateRenderNoVars(t *testing.T) {
	tool := tools.TemplateRenderTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"template": "Static text",
		"format":   "text",
	}, nil)
	if err != nil {
		t.Fatalf("template_render: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Static text") {
		t.Errorf("expected 'Static text', got: %s", result.Content[0].Text)
	}
}
