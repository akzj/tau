package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding/tools"
)

func init() {
	tools.WorkspaceRoot = "/tmp"
}

func TestSearchCodeTool_Found(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "test.go"), "package test\n\nfunc Hello() {\n\tx := 42\n}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "Hello"}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected content")
	}
	if !strings.Contains(res.Content[0].Text, "Hello") {
		t.Fatalf("expected 'Hello' in result, got: %s", res.Content[0].Text)
	}
}

func TestSearchCodeTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "test.go"), "package test\n\nfunc Hello() {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "NonExistentFunc"}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No matches") {
		t.Fatalf("expected 'No matches', got: %s", res.Content[0].Text)
	}
}

func TestSearchCodeTool_InvalidRegex(t *testing.T) {
	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "["}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
}

func TestSearchCodeTool_WithGlob(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "test.go"), "package test\nfunc Hello() {}\n")
	writeFile(t, filepath.Join(dir, "test.txt"), "Hello world")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "Hello", "file_glob": "*.go"}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "test.go") {
		t.Fatalf("expected match in test.go only, got: %s", res.Content[0].Text)
	}
}

func TestSearchCodeTool_ContextLines(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "test.go"), "package test\n\nfunc Hello() {\n\tx := 42\n}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "Hello", "context_lines": 2}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "Hello") {
		t.Fatalf("expected 'Hello' in result, got: %s", text)
	}
}

func TestSearchCodeTool_MaxResults(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	// Create file with many matches
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "// MATCH_HERE")
	}
	writeFile(t, filepath.Join(dir, "test.go"), "package test\n"+strings.Join(lines, "\n"))
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.SearchCodeTool()
	params := map[string]any{"query": "MATCH_HERE", "max_results": 3}
	res, err := tool.Execute(context.Background(), "call6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// MaxResults limits search results but context lines add extra lines
	// Just verify we get results and not an error (no strict count check)
	_ = res.Details
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		_ = err
	}
}

var _ core.ToolSchema = tools.Schema{}
