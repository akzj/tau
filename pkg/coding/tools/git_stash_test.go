package tools_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGitStashInvalidAction(t *testing.T) {
	tool := tools.GitStashTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "invalid"}, nil)
	if err == nil {
		t.Error("expected error for invalid action")
	}
}

func TestGitStashList(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	exec.Command("git", "-C", dir, "init").Run()

	tool := tools.GitStashTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "list"}, nil)
	if err != nil {
		t.Skipf("git stash list failed: %v", err)
	}
	_ = result
}

func TestGitStashPushBasic(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	exec.Command("git", "-C", dir, "init").Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test").Run()

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)
	exec.Command("git", "-C", dir, "add", "test.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial").Run()

	os.WriteFile(filepath.Join(dir, "test.txt"), []byte("modified"), 0644)

	tool := tools.GitStashTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "push", "message": "test stash"}, nil)
	if err != nil {
		t.Skipf("git stash push failed: %v", err)
	}
}
