package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGitRevertTool_MissingCommit(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitRevertTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing commit")
	}
}

func TestGitRevertTool_RevertCommit(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	// Create a commit to revert
	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", dir, "add", "test.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "add test.txt").Run()

	// Get the commit hash
	out, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput()
	hash := string(out)[:8] // short hash

	tool := GitRevertTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"commit": string(hash), "no_edit": true,
	}, nil)
	if err != nil {
		t.Fatalf("revert failed: %v", err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success, got: %v", result.Details)
	}
}
