package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestExtractFunction_Basic(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func compute() int {
	x := 1
	y := 2
	z := x + y
	return z
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ExtractFunctionTool()
	params := map[string]any{
		"file":              "main.go",
		"start_line":        5,
		"end_line":          6,
		"new_function_name": "addValues",
	}
	res, err := tool.Execute(context.Background(), "ef1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Extracted lines") {
		t.Errorf("expected extraction summary, got:\n%s", text)
	}
	if !strings.Contains(text, "addValues") {
		t.Errorf("expected addValues function, got:\n%s", text)
	}

	// Verify file was modified
	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if !strings.Contains(str, "func addValues()") {
		t.Errorf("expected addValues function definition, got:\n%s", str)
	}
	if !strings.Contains(str, "addValues()") {
		t.Errorf("expected addValues() call site, got:\n%s", str)
	}
}

func TestExtractFunction_MultiLine(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

func process() string {
	a := "hello"
	b := "world"
	c := a + " " + b
	return c
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ExtractFunctionTool()
	params := map[string]any{
		"file":              "main.go",
		"start_line":        5,
		"end_line":          7,
		"new_function_name": "concat",
	}
	res, err := tool.Execute(context.Background(), "ef2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "concat") {
		t.Errorf("expected concat function, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if !strings.Contains(str, "func concat()") {
		t.Errorf("expected concat definition:\n%s", str)
	}
	if !strings.Contains(str, "concat()") {
		t.Errorf("expected concat call:\n%s", str)
	}
}

func TestExtractFunction_InvalidName(t *testing.T) {
	tool := tools.ExtractFunctionTool()

	params := map[string]any{
		"file":              "x.go",
		"start_line":        1,
		"end_line":          2,
		"new_function_name": "123bad",
	}
	_, err := tool.Execute(context.Background(), "ef3", params, nil)
	if err == nil || !strings.Contains(err.Error(), "valid Go identifier") {
		t.Errorf("expected identifier error, got: %v", err)
	}
}

func TestExtractFunction_LineOrderError(t *testing.T) {
	tool := tools.ExtractFunctionTool()

	params := map[string]any{
		"file":              "x.go",
		"start_line":        10,
		"end_line":          5,
		"new_function_name": "testFunc",
	}
	_, err := tool.Execute(context.Background(), "ef4", params, nil)
	if err == nil || !strings.Contains(err.Error(), "start_line must be less than") {
		t.Errorf("expected line order error, got: %v", err)
	}
}

func TestExtractFunction_MissingParams(t *testing.T) {
	tool := tools.ExtractFunctionTool()

	_, err := tool.Execute(context.Background(), "ef5", map[string]any{
		"file": "", "start_line": 1, "end_line": 2, "new_function_name": "f",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}

	_, err = tool.Execute(context.Background(), "ef6", map[string]any{
		"file": "x.go", "start_line": 0, "end_line": 2, "new_function_name": "f",
	}, nil)
	if err == nil {
		t.Fatal("expected error for start_line <= 0")
	}

	_, err = tool.Execute(context.Background(), "ef7", map[string]any{
		"file": "x.go", "start_line": 1, "end_line": 2, "new_function_name": "",
	}, nil)
	if err == nil {
		t.Fatal("expected error for missing new_function_name")
	}
}

func TestExtractFunction_FileNotFound(t *testing.T) {
	tool := tools.ExtractFunctionTool()
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	params := map[string]any{
		"file":              "nonexistent.go",
		"start_line":        1,
		"end_line":          2,
		"new_function_name": "testFunc",
	}
	_, err := tool.Execute(context.Background(), "ef8", params, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
