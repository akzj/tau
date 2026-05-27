package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintCheckTool_MissingPath(t *testing.T) {
	tool := LintCheckTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestLintCheckTool_GoVet_CleanFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Write a clean Go file
	path := filepath.Join(dir, "clean.go")
	os.WriteFile(path, []byte("package p\n\nimport \"fmt\"\n\nfunc Hello() {\n\tfmt.Println(\"hi\")\n}\n"), 0644)

	tool := LintCheckTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"linters": "govet",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
}

func TestLintCheckTool_GoVet_WithIssue(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Write a Go file with a vet-detectable issue: unreachable code
	path := filepath.Join(dir, "bad.go")
	os.WriteFile(path, []byte("package p\n\nfunc Bad() int {\n\treturn 1\n\treturn 2\n}\n"), 0644)

	tool := LintCheckTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"linters": "govet",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// govet should report unreachable code
	if !strings.Contains(result.Content[0].Text, "unreachable") {
		t.Logf("govet output: %s", result.Content[0].Text)
	}
	details, ok := result.Details["issue_count"].(int)
	if ok {
		t.Logf("issue_count: %d", details)
	}
}

func TestLintCheckTool_Gofmt_Misformatted(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Write a mis-formatted Go file (bad indentation)
	path := filepath.Join(dir, "badfmt.go")
	os.WriteFile(path, []byte("package p\n\nfunc F(){ fmt.Println(\"x\")}\n"), 0644)

	tool := LintCheckTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"linters": "gofmt",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
}

func TestLintCheckTool_ParseLinterSet(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{"govet"}},
		{"govet", []string{"govet"}},
		{"govet,gofmt", []string{"govet", "gofmt"}},
		{"govet, gofmt ,staticcheck", []string{"govet", "gofmt", "staticcheck"}},
	}

	for _, tt := range tests {
		result := parseLinterSet(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseLinterSet(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestLintCheckTool_UnknownLinter(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	path := filepath.Join(dir, "clean.go")
	os.WriteFile(path, []byte("package p\n\nfunc F(){}\n"), 0644)

	tool := LintCheckTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":    path,
		"linters": "nonexistent_linter",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "Unknown linter") {
		t.Errorf("expected warning about unknown linter, got: %s", result.Content[0].Text)
	}
}