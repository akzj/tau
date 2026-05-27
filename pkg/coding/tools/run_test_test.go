package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestRunTestTool_Pass(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "math_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunTestTool()
	params := map[string]any{"path": "./...", "timeout": "30s"}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "[pass]") {
		t.Fatalf("expected [pass], got: %s", res.Content[0].Text)
	}
	passed, ok := res.Details["passed"].(bool)
	if !ok || !passed {
		t.Fatal("expected passed=true")
	}
}

func TestRunTestTool_Fail(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "fail_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestFail(t *testing.T) { t.Fatal(\"boom\") }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunTestTool()
	params := map[string]any{"path": "./...", "timeout": "30s"}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	passed, ok := res.Details["passed"].(bool)
	if !ok || passed {
		t.Fatal("expected passed=false")
	}
}

func TestRunTestTool_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "math_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunTestTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "[pass]") {
		t.Fatalf("expected [pass], got: %s", res.Content[0].Text)
	}
}

func TestRunTestTool_Verbose(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "math_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunTestTool()
	params := map[string]any{"path": "./...", "timeout": "30s", "verbose": true}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "[pass]") {
		t.Fatalf("expected [pass], got: %s", res.Content[0].Text)
	}
}

func TestRunTestTool_BadTimeout(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "math_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestOK(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunTestTool()
	params := map[string]any{"path": "./...", "timeout": "bad"}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Should still pass with default timeout
	if !strings.Contains(res.Content[0].Text, "[pass]") {
		t.Fatalf("expected [pass], got: %s", res.Content[0].Text)
	}
}

func init() {
	_ = tools.Schema{}
}
