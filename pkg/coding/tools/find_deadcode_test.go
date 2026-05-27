package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestFindDeadcode_UnusedFunction(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	// main.go: defines unusedHelper(), never called.
	writeFile(t, filepath.Join(dir, "main.go"), `package main

func unusedHelper() string {
	return "unused"
}

func main() {
	println("ok")
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()
	params := map[string]any{"path": ".", "include_tests": false}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "unusedHelper") {
		t.Fatalf("expected 'unusedHelper' flagged as dead, got:\n%s", text)
	}
}

func TestFindDeadcode_AllUsed(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func helper() string {
	return "ok"
}

func main() {
	println(helper())
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()
	params := map[string]any{"path": ".", "include_tests": false}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if strings.Contains(text, "helper()") && strings.Contains(text, "never called") {
		t.Fatalf("expected 'helper' to NOT be flagged as dead, but it was:\n%s", text)
	}
	if !strings.Contains(text, "No potentially unused functions") {
		t.Fatalf("expected 'No potentially unused functions', got:\n%s", text)
	}
}

func TestFindDeadcode_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()
	params := map[string]any{"path": ".", "include_tests": false}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "Dead Code Report") {
		t.Fatalf("expected report header, got:\n%s", text)
	}
}

func TestFindDeadcode_IncludeTests(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	// main.go: defines helper(), NOT called from non-test files.
	writeFile(t, filepath.Join(dir, "main.go"), `package main

func helper() string {
	return "ok"
}

func main() {
	println("ok")
}
`)

	// main_test.go: calls helper().
	writeFile(t, filepath.Join(dir, "main_test.go"), `package main

import "testing"

func TestHelper(t *testing.T) {
	got := helper()
	if got != "ok" {
		t.Fatal("unexpected")
	}
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()

	// Without include_tests: helper should appear dead (only called from test file).
	paramsNoTests := map[string]any{"path": ".", "include_tests": false}
	resNo, err := tool.Execute(context.Background(), "call4a", paramsNoTests, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	textNo := resNo.Content[0].Text
	if !strings.Contains(textNo, "helper() — never called") {
		t.Fatalf("without include_tests, expected 'helper' flagged as dead, got:\n%s", textNo)
	}

	// With include_tests: helper should be found as called.
	paramsWithTests := map[string]any{"path": ".", "include_tests": true}
	resWith, err := tool.Execute(context.Background(), "call4b", paramsWithTests, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	textWith := resWith.Content[0].Text
	if strings.Contains(textWith, "helper() — never called") {
		t.Fatalf("with include_tests, expected 'helper' to NOT be dead, got:\n%s", textWith)
	}
}

func TestFindDeadcode_IgnoreInitAndMain(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func init() { println("init") }
func main() { println("main") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()
	params := map[string]any{"path": ".", "include_tests": false}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if strings.Contains(text, "init() — never called") || strings.Contains(text, "main() — never called") {
		t.Fatalf("init and main should never be flagged as dead, got:\n%s", text)
	}
}

func TestFindDeadcode_ResolvePath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "src", "main.go"), `package main

func unusedHelper() {}
func main() { println("ok") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.FindDeadcodeTool()
	params := map[string]any{"path": "src", "include_tests": false}
	res, err := tool.Execute(context.Background(), "call6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "unusedHelper") {
		t.Fatalf("expected 'unusedHelper' flagged in src subdirectory, got:\n%s", text)
	}
}

func init() {
	_ = os.MkdirAll
}
