package tools_test

import (
	"context"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGitHubSearchNoToken(t *testing.T) {
	tool := tools.GitHubSearchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "golang"}, nil)
	if err == nil {
		t.Error("expected error when GITHUB_TOKEN not set")
	}
}

func TestGitHubSearchInvalidType(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	tool := tools.GitHubSearchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "test", "type": "invalid"}, nil)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}
