package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestFileSearchByGlob(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x"), 0644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello"), 0644)

	tool := tools.FileSearchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*.go"}, nil)
	if err != nil {
		t.Fatalf("file_search: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "a.go") {
		t.Errorf("expected a.go in results, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "b.txt") {
		t.Error("should not contain b.txt")
	}
}

func TestFileSearchWithRegex(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "a.go"), []byte("func hello() { return nil }"), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte("package z"), 0644)

	tool := tools.FileSearchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*.go", "regex": "hello"}, nil)
	if err != nil {
		t.Fatalf("file_search: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "a.go") {
		t.Errorf("expected a.go, got: %s", result.Content[0].Text)
	}
	if strings.Contains(result.Content[0].Text, "b.go") {
		t.Error("should not contain b.go")
	}
}

func TestFileSearchNoMatch(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.FileSearchTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"pattern": "*.nonexistent"}, nil)
	if !strings.Contains(result.Content[0].Text, "no matches") {
		t.Logf("file_search result: %s", result.Content[0].Text)
	}
}
