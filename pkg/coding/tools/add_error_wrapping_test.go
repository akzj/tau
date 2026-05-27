package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestAddErrorWrapping_Basic(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func readFile(name string) error {
	_, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	return nil
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.AddErrorWrappingTool()
	params := map[string]any{
		"file": "main.go",
	}
	res, err := tool.Execute(context.Background(), "aew1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Wraps added: 1") {
		t.Errorf("expected 1 wrap, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if !strings.Contains(str, "fmt.Errorf") {
		t.Errorf("expected fmt.Errorf wrapping, got:\n%s", str)
	}
	if !strings.Contains(str, "%w") {
		t.Errorf("expected %%w verb for error wrapping, got:\n%s", str)
	}
}

func TestAddErrorWrapping_MultipleReturns(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func process() error {
	_, err := os.ReadFile("a.txt")
	if err != nil {
		return err
	}
	_, err = os.ReadFile("b.txt")
	if err != nil {
		return err
	}
	return nil
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.AddErrorWrappingTool()
	params := map[string]any{
		"file": "main.go",
	}
	res, err := tool.Execute(context.Background(), "aew2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Wraps added: 2") {
		t.Errorf("expected 2 wraps, got:\n%s", text)
	}
}

func TestAddErrorWrapping_WithPattern(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func process() error {
	_, err := os.ReadFile("a.txt")
	if err != nil {
		return err
	}
	return fmt.Errorf("process: %w", err)
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.AddErrorWrappingTool()
	params := map[string]any{
		"file":    "main.go",
		"pattern": `return err\b`,
	}
	res, err := tool.Execute(context.Background(), "aew3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Wraps added: 1") {
		t.Errorf("expected 1 wrap matching pattern, got:\n%s", text)
	}

	// The already-wrapped line should not be double-wrapped
	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	count := strings.Count(str, "fmt.Errorf")
	if count != 2 {
		t.Errorf("expected 2 fmt.Errorf (1 original + 1 new), got %d:\n%s", count, str)
	}
}

func TestAddErrorWrapping_NoBareReturns(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "os"

func process() error {
	_, err := os.ReadFile("a.txt")
	if err != nil {
		return err
	}
	return nil
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.AddErrorWrappingTool()
	params := map[string]any{
		"file": "main.go",
	}
	res, err := tool.Execute(context.Background(), "aew4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Wraps added: 1") {
		t.Errorf("expected 1 wrap, got:\n%s", text)
	}

	// Re-run on same file: should add 0 wraps
	res2, err := tool.Execute(context.Background(), "aew4b", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text2 := res2.Content[0].Text
	if !strings.Contains(text2, "Wraps added: 0") {
		t.Errorf("second run: expected 0 wraps, got:\n%s", text2)
	}
}

func TestAddErrorWrapping_MissingFile(t *testing.T) {
	tool := tools.AddErrorWrappingTool()
	params := map[string]any{"file": ""}
	_, err := tool.Execute(context.Background(), "aew5", params, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestAddErrorWrapping_InvalidRegex(t *testing.T) {
	tool := tools.AddErrorWrappingTool()
	params := map[string]any{"file": "x.go", "pattern": "["}
	_, err := tool.Execute(context.Background(), "aew6", params, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid regex") {
		t.Errorf("expected invalid regex error, got: %v", err)
	}
}
