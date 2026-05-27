package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestFileDiffDirBasic(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	base := t.TempDir()
	tools.WorkspaceRoot = base

	dirA := filepath.Join(base, "dir_a")
	dirB := filepath.Join(base, "dir_b")
	os.MkdirAll(dirA, 0755)
	os.MkdirAll(dirB, 0755)

	os.WriteFile(filepath.Join(dirA, "shared.txt"), []byte("same"), 0644)
	os.WriteFile(filepath.Join(dirB, "shared.txt"), []byte("same"), 0644)

	os.WriteFile(filepath.Join(dirA, "only_a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dirB, "only_b.txt"), []byte("b"), 0644)

	tool := tools.FileDiffDirTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"dir_a": "dir_a", "dir_b": "dir_b"}, nil)
	if err != nil {
		t.Fatalf("file_diff_dir: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "only_a.txt") {
		t.Errorf("expected only_a.txt, got: %s", text)
	}
	if !strings.Contains(text, "only_b.txt") {
		t.Errorf("expected only_b.txt, got: %s", text)
	}
}

func TestFileDiffDirWithFilter(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	base := t.TempDir()
	tools.WorkspaceRoot = base

	dirA := filepath.Join(base, "dir_a")
	dirB := filepath.Join(base, "dir_b")
	os.MkdirAll(dirA, 0755)
	os.MkdirAll(dirB, 0755)

	os.WriteFile(filepath.Join(dirA, "x.go"), []byte("package x"), 0644)
	os.WriteFile(filepath.Join(dirB, "y.go"), []byte("package y"), 0644)

	tool := tools.FileDiffDirTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"dir_a": "dir_a", "dir_b": "dir_b", "filter": "*.go"}, nil)
	if err != nil {
		t.Fatalf("file_diff_dir: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "x.go") {
		t.Errorf("expected x.go, got: %s", text)
	}
}
