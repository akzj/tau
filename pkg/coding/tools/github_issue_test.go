package tools_test

import (
	"context"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGitHubIssueNoToken(t *testing.T) {
	tool := tools.GitHubIssueTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "list", "repo": "golang/go"}, nil)
	if err == nil {
		t.Error("expected error when GITHUB_TOKEN not set")
	}
}

func TestGitHubIssueInvalidAction(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	tool := tools.GitHubIssueTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "invalid", "repo": "golang/go"}, nil)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}
