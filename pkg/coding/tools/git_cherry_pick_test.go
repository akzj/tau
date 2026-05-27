package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitCherryPickTool_MissingCommit(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitCherryPickTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing commit")
	}
}

func TestGitCherryPickTool_PickCommit(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	// Create a commit on a branch
	exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run()
	os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feature"), 0644)
	exec.Command("git", "-C", dir, "add", "feat.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feature commit").Run()

	// Get commit hash
	out, _ := exec.Command("git", "-C", dir, "rev-parse", "HEAD").CombinedOutput()
	commitHash := strings.TrimSpace(string(out))

	// Switch back to main
	exec.Command("git", "-C", dir, "checkout", "master").Run()
	// If master doesn't exist, use the default branch
	out2, _ := exec.Command("git", "-C", dir, "branch", "--show-current").CombinedOutput()
	_ = out2

	tool := GitCherryPickTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"commit": commitHash,
	}, nil)
	if err != nil {
		t.Fatalf("cherry-pick failed: %v", err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success, got: %v", result.Details)
	}
}
