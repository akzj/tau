package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChangelogTool_MissingFrom(t *testing.T) {
	tool := ChangelogTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing from version")
	}
}

func TestChangelogTool_KeepAChangelogFormat(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()
	os.WriteFile(filepath.Join(dir, "newapi.go"), []byte("package main"), 0644)
	exec.Command("git", "-C", dir, "add", "newapi.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: add new API").Run()
	exec.Command("git", "-C", dir, "tag", "v1.1.0").Run()

	tool := ChangelogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from":    "v1.0.0",
		"to":      "v1.1.0",
		"version": "v1.1.0",
	}, nil)
	if err != nil {
		t.Fatalf("changelog: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "## [v1.1.0]") {
		t.Errorf("expected version heading, got: %s", text)
	}
	if !strings.Contains(text, "Added") {
		t.Errorf("expected 'Added' section, got: %s", text)
	}
}

func TestChangelogTool_MarkdownFormat(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v2.0.0").Run()
	os.WriteFile(filepath.Join(dir, "fix.txt"), []byte("fix"), 0644)
	exec.Command("git", "-C", dir, "add", "fix.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "fix: resolve bug").Run()
	exec.Command("git", "-C", dir, "tag", "v2.0.1").Run()

	tool := ChangelogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from":   "v2.0.0",
		"to":     "v2.0.1",
		"format": "markdown",
	}, nil)
	if err != nil {
		t.Fatalf("changelog markdown: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Fixed") {
		t.Errorf("expected 'Fixed' section, got: %s", text)
	}
}

func TestChangelogTool_EmptyRange(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v3.0.0").Run()
	// No commits between tag and itself

	tool := ChangelogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from":    "v3.0.0",
		"to":      "v3.0.0",
		"version": "v3.0.0",
	}, nil)
	if err != nil {
		t.Fatalf("empty changelog: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "No changes recorded") {
		t.Errorf("expected 'No changes recorded', got: %s", text)
	}
}

func TestChangelogTool_SecurityCommits(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v4.0.0").Run()
	os.WriteFile(filepath.Join(dir, "patch.txt"), []byte("security fix"), 0644)
	exec.Command("git", "-C", dir, "add", "patch.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "security: fix CVE-2024-0001").Run()
	exec.Command("git", "-C", dir, "tag", "v4.0.1").Run()

	tool := ChangelogTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from": "v4.0.0",
		"to":   "v4.0.1",
	}, nil)
	if err != nil {
		t.Fatalf("changelog security: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Security") {
		t.Errorf("expected 'Security' section, got: %s", text)
	}
}