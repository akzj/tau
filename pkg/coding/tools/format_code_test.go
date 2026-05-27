package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestFormatCodeTool_DryRunIssues(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	// Write improperly formatted Go code (extra spaces)
	writeFile(t, filepath.Join(dir, "unformatted.go"), "package test\n\nfunc  Foo(  )  { }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FormatCodeTool()
	params := map[string]any{"path": dir, "write": false}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Should show diff or "No formatting issues"
	text := res.Content[0].Text
	if text == "" {
		t.Fatal("expected output, got empty")
	}
}

func TestFormatCodeTool_DryRunNoIssues(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "good.go"), "package test\n\nfunc Foo() {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FormatCodeTool()
	params := map[string]any{"path": dir, "write": false}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "No formatting issues") {
		t.Fatalf("expected 'No formatting issues', got: %s", text)
	}
}

func TestFormatCodeTool_Write(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "unformatted.go"), "package test\n\nfunc  Foo(  )  { }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.FormatCodeTool()
	params := map[string]any{"path": filepath.Join(dir, "unformatted.go"), "write": true}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	// Write mode may output diff or confirmation depending on tool
	if text == "" {
		t.Fatal("expected output, got empty")
	}
}

func TestFormatCodeTool_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "good.go"), "package test\n\nfunc Foo() {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.FormatCodeTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "No formatting issues") {
		t.Fatalf("expected 'No formatting issues', got: %s", text)
	}
}

func TestFormatCodeTool_EmptyPathFallsBack(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "good.go"), "package test\n\nfunc Foo() {}\n")

	tool := tools.FormatCodeTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	_ = res
}

func init() {
	_ = os.Stat
}
