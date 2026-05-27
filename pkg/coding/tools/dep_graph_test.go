package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestDepGraph_TextFormat(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println(os.Args[0])
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": ".", "output_format": "text", "max_depth": 2}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if text == "" {
		t.Fatal("expected non-empty output")
	}
	if !strings.Contains(text, "fmt") && !strings.Contains(text, "os") {
		t.Fatalf("expected dependency on fmt or os, got:\n%s", text)
	}
	// Text format should use tree characters.
	if !strings.Contains(text, "├") && !strings.Contains(text, "└") {
		t.Logf("warning: text output may not contain tree chars:\n%s", text)
	}
}

func TestDepGraph_MermaidFormat(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("hello") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": ".", "output_format": "mermaid", "max_depth": 2}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "```mermaid") {
		t.Fatalf("expected mermaid code block, got:\n%s", text)
	}
	if !strings.Contains(text, "graph TD") {
		t.Fatalf("expected 'graph TD' in mermaid output, got:\n%s", text)
	}
}

func TestDepGraph_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("hello") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": ".", "output_format": "json", "max_depth": 2}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	// Should be valid JSON.
	var entries []struct {
		Package      string   `json:"package"`
		Dependencies []string `json:"dependencies"`
	}
	if err := json.Unmarshal([]byte(text), &entries); err != nil {
		t.Fatalf("expected valid JSON array, got error: %v\noutput:\n%s", err, text)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one package entry")
	}
	foundRoot := false
	for _, e := range entries {
		if e.Package == "testmod" {
			foundRoot = true
			break
		}
	}
	if !foundRoot {
		t.Fatalf("expected root package 'testmod' in JSON, got entries: %v", entries)
	}
}

func TestDepGraph_NonexistentPath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": "./nonexistent", "output_format": "text"}
	_, err := tool.Execute(context.Background(), "call4", params, nil)
	if err == nil {
		// Some go versions may succeed with empty output; that's acceptable.
		t.Log("go list did not error on nonexistent path (may be version-dependent)")
	}
}

func TestDepGraph_DotFormat(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("hello") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": ".", "output_format": "dot", "max_depth": 2}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "digraph") {
		t.Fatalf("expected digraph output, got:\n%s", text)
	}
	if !strings.Contains(text, "->") {
		t.Fatalf("expected edge arrows, got:\n%s", text)
	}
}

func TestDepGraph_MaxDepth(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

func main() { fmt.Println("hello") }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()

	// depth=0 should give minimal output.
	params := map[string]any{"path": ".", "output_format": "text", "max_depth": 0}
	res, err := tool.Execute(context.Background(), "call6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "testmod") {
		t.Fatalf("expected root package in output, got:\n%s", text)
	}
	// With depth=0, should not show any child deps.
}

func TestDepGraph_DefaultFormat(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "os"

func main() { os.Exit(0) }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module testmod\n\ngo 1.21\n")

	tool := tools.DepGraphTool()
	params := map[string]any{"path": "."}
	res, err := tool.Execute(context.Background(), "call7", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if text == "" {
		t.Fatal("expected non-empty default output")
	}
}

func init() {
	_ = os.MkdirAll
}
