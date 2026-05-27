package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestInlineVariable_Simple(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func compute() int {
	x := 42
	return x
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.InlineVariableTool()
	params := map[string]any{
		"file":          "main.go",
		"variable_name": "x",
	}
	res, err := tool.Execute(context.Background(), "iv1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Inlined variable: x") {
		t.Errorf("expected inline summary, got:\n%s", text)
	}
	if !strings.Contains(text, "42") {
		t.Errorf("expected initializer '42' in output, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if strings.Contains(str, "x := 42") || strings.Contains(str, "x = 42") {
		t.Errorf("declaration of x still present:\n%s", str)
	}
	if !strings.Contains(str, "42") {
		t.Errorf("expected 42 still present, got:\n%s", str)
	}
}

func TestInlineVariable_StringLiteral(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func greet() string {
	msg := "hello world"
	return msg
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.InlineVariableTool()
	params := map[string]any{
		"file":          "main.go",
		"variable_name": "msg",
	}
	res, err := tool.Execute(context.Background(), "iv2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, `"hello world"`) {
		t.Errorf("expected initializer in output, got:\n%s", text)
	}
}

func TestInlineVariable_MultipleRefs(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func compute() int {
	n := 100
	a := n * 2
	b := n + 10
	return a + b + n
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.InlineVariableTool()
	params := map[string]any{
		"file":          "main.go",
		"variable_name": "n",
	}
	res, err := tool.Execute(context.Background(), "iv3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "References inlined: 3") {
		t.Errorf("expected 3 references inlined, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if strings.Contains(str, "n := 100") {
		t.Errorf("n declaration still present:\n%s", str)
	}
}

func TestInlineVariable_NotFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func compute() int {
	return 1
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.InlineVariableTool()
	params := map[string]any{
		"file":          "main.go",
		"variable_name": "nonExistent",
	}
	_, err := tool.Execute(context.Background(), "iv4", params, nil)
	if err == nil {
		t.Fatal("expected error for non-existent variable")
	}
}

func TestInlineVariable_MissingParams(t *testing.T) {
	tool := tools.InlineVariableTool()

	_, err := tool.Execute(context.Background(), "iv5", map[string]any{"file": "", "variable_name": "x"}, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	_, err = tool.Execute(context.Background(), "iv6", map[string]any{"file": "x.go", "variable_name": ""}, nil)
	if err == nil {
		t.Fatal("expected error for missing variable_name")
	}
}

func TestInlineVariable_NoRefs(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func compute() {
	x := 1
	y := x + 1
	_ = y
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.InlineVariableTool()
	params := map[string]any{
		"file":          "main.go",
		"variable_name": "y",
	}
	res, err := tool.Execute(context.Background(), "iv7", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Inlined variable: y") {
		t.Errorf("expected inline summary, got:\n%s", text)
	}
}
