package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestDocGenerateFile(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	goCode := `package test

// Hello greets the world.
func Hello() string {
	return "hello"
}

// Add adds two integers.
func Add(a, b int) int {
	return a + b
}
`
	os.WriteFile(filepath.Join(dir, "test.go"), []byte(goCode), 0644)

	tool := tools.DocGenerateTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"path": "test.go"}, nil)
	if err != nil {
		t.Fatalf("doc_generate: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Hello") || !strings.Contains(text, "Add") {
		t.Errorf("expected function signatures, got: %s", text)
	}
	if !strings.Contains(text, "greets the world") {
		t.Errorf("expected comment, got: %s", text)
	}
}

func TestDocGenerateDir(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	goCode := `package test

// Foo does something.
func Foo() {}
`
	os.WriteFile(filepath.Join(dir, "foo.go"), []byte(goCode), 0644)

	tool := tools.DocGenerateTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatalf("doc_generate: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Foo") {
		t.Errorf("expected Foo, got: %s", result.Content[0].Text)
	}
}

func TestDocGenerateNonGoFile(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("not go"), 0644)

	tool := tools.DocGenerateTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"path": "notes.txt"}, nil)
	if err == nil {
		t.Error("expected error for non-.go file")
	}
}
