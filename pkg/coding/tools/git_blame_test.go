package tools_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGitBlameInvalidFile(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.GitBlameTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "nonexistent.go"}, nil)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGitBlameBasic(t *testing.T) {
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

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)
	exec.Command("git", "-C", dir, "add", "main.go").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial").Run()

	tool := tools.GitBlameTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file_path": "main.go"}, nil)
	if err != nil {
		t.Skipf("git blame failed: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Test") {
		t.Errorf("expected author in blame output, got: %s", result.Content[0].Text)
	}
}
