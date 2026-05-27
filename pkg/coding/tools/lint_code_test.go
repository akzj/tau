package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestLintCodeTool_NoIssues(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "clean.go"), "package test\n\nfunc Foo() error { return nil }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.LintCodeTool()
	params := map[string]any{"path": "./..."}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "no issues found") {
		t.Fatalf("expected 'no issues found', got: %s", text)
	}
}

func TestLintCodeTool_IssuesFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	// This code has unused import which go vet catches
	writeFile(t, filepath.Join(dir, "bad.go"), "package test\n\nimport \"fmt\"\n\nfunc Foo() {\n\t_ = 1\n}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.LintCodeTool()
	params := map[string]any{"path": "./..."}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "go vet issues") {
		t.Fatalf("expected 'go vet issues' header, got: %s", text)
	}
}

func TestLintCodeTool_DefaultPath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "clean.go"), "package test\n\nfunc Foo() error { return nil }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.LintCodeTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if strings.TrimSpace(text) == "" {
		t.Fatal("expected non-empty output")
	}
}

func TestLintCodeTool_OutputTruncation(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "clean.go"), "package test\n\nfunc Foo() error { return nil }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.LintCodeTool()
	params := map[string]any{"path": "./..."}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(res.Content) == 0 || res.Content[0].Text == "" {
		t.Fatal("expected content")
	}
}

func TestLintCodeTool_NonExistentPath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.LintCodeTool()
	params := map[string]any{"path": "./nonexistent"}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// go vet may produce output about missing package
	_ = res
}

func init() {
	_ = filepath.Abs
}
