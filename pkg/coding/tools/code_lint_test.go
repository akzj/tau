package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCodeLintTool_MissingPath(t *testing.T) {
	tool := CodeLintTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestCodeLintTool_LintFile(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	// Write a simple Go file with no issues
	path := filepath.Join(dir, "clean.go")
	os.WriteFile(path, []byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)

	tool := CodeLintTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Errorf("expected text content")
	}
}
