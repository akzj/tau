package tools

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestCIStatusTool_MissingRepo(t *testing.T) {
	tool := CIStatusTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing repo")
	}
}

func TestCIStatusTool_UnknownProvider(t *testing.T) {
	tool := CIStatusTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":     "test/repo",
		"provider": "unknown",
	}, nil)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' error, got: %v", err)
	}
}

func TestCIStatusTool_LocalProvider(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := CIStatusTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":     "local-test",
		"provider": "local",
	}, nil)
	if err != nil {
		t.Fatalf("local CI check should not fail: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Local CI Check") {
		t.Errorf("expected 'Local CI Check' in output, got: %s", text)
	}
}

func TestCIStatusTool_GitHubProviderNoToken(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir
	os.Unsetenv("GITHUB_TOKEN")

	tool := CIStatusTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":     "golang/go",
		"provider": "github",
		"ref":      "master",
	}, nil)
	if err != nil {
		t.Fatalf("GitHub CI check should not return error even without token: %v", err)
	}
	// Should fall back to local git check
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestCIStatusTool_DefaultProvider(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir
	os.Unsetenv("GITHUB_TOKEN")

	tool := CIStatusTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo": "golang/go",
	}, nil)
	if err != nil {
		t.Fatalf("default provider should not fail: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}