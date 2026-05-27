package tools

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func TestPRReviewTool_MissingRepo(t *testing.T) {
	tool := PRReviewTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"pr_number": 1}, nil)
	if err == nil {
		t.Error("expected error for missing repo")
	}
}

func TestPRReviewTool_MissingPRNumber(t *testing.T) {
	tool := PRReviewTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"repo": "test/repo"}, nil)
	if err == nil {
		t.Error("expected error for missing pr_number")
	}
}

func TestPRReviewTool_LocalReview(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := PRReviewTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":        "local/repo",
		"pr_number":   1,
		"source":      "local",
		"auto_approve": false,
	}, nil)
	if err != nil {
		t.Fatalf("local PR review: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "PR Review") {
		t.Errorf("expected 'PR Review' in output, got: %s", text)
	}
}

func TestPRReviewTool_LocalReviewWithChanges(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	// Make an uncommitted change
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "base").Run()

	tool := PRReviewTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":      "local/repo",
		"pr_number": 1,
		"source":    "local",
	}, nil)
	if err != nil {
		t.Fatalf("local PR review with changes: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestPRReviewTool_GitHubNoToken(t *testing.T) {
	tool := PRReviewTool()
	// Without GITHUB_TOKEN, will try GitHub API and may fail gracefully
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"repo":      "nonexistent/repo",
		"pr_number": 99999,
		"source":    "github",
	}, nil)
	// May error from GitHub API (expected for nonexistent repo)
	if err != nil {
		return // acceptable — network error or 404
	}
	if len(result.Content) == 0 {
		t.Error("expected content even on error")
	}
}