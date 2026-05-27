package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestFindReferencesTool_Found(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "app.go"), "package main\n\nfunc Hello() {}\n\nfunc main() {\n\tHello()\n}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FindReferencesTool()
	params := map[string]any{"symbol": "Hello", "path": dir}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "Hello") {
		t.Fatalf("expected 'Hello' in result, got: %s", res.Content[0].Text)
	}
}

func TestFindReferencesTool_NotFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "app.go"), "package main\nfunc main() {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FindReferencesTool()
	params := map[string]any{"symbol": "NonExistentFunc", "path": dir}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No references") {
		t.Fatalf("expected 'No references', got: %s", res.Content[0].Text)
	}
}

func TestFindReferencesTool_EmptySymbol(t *testing.T) {
	tool := tools.FindReferencesTool()
	params := map[string]any{"symbol": ""}
	_, err := tool.Execute(context.Background(), "call3", params, nil)
	if err == nil {
		t.Fatal("expected error for empty symbol")
	}
}

func TestFindReferencesTool_NonexistentPath(t *testing.T) {
	tools.WorkspaceRoot = "/tmp"
	tool := tools.FindReferencesTool()
	params := map[string]any{"symbol": "Hello", "path": "/nonexistent/path"}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	// grep fails on nonexistent path, tool returns message
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No references") {
		t.Fatalf("expected 'No references', got: %s", res.Content[0].Text)
	}
}

func TestFindReferencesTool_EmptyPathUsesWorkspace(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "main.go"), "package main\n\nfunc MyFunc() {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FindReferencesTool()
	params := map[string]any{"symbol": "MyFunc"}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "MyFunc") {
		t.Fatalf("expected 'MyFunc' in result, got: %s", res.Content[0].Text)
	}
}

func init() {
	_ = os.MkdirAll
}
