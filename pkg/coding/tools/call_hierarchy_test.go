package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestCallHierarchy_Callees(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func helper() string {
	return "help"
}

func wrapper() string {
	return helper()
}

func outer() string {
	x := helper()
	return wrapper() + x
}

func main() {
	outer()
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.CallHierarchyTool()
	params := map[string]any{
		"function_name": "outer",
		"direction":     "callees",
		"max_depth":     2,
	}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "helper") {
		t.Errorf("expected 'helper' in callees, got:\n%s", text)
	}
	if !strings.Contains(text, "wrapper") {
		t.Errorf("expected 'wrapper' in callees, got:\n%s", text)
	}
}

func TestCallHierarchy_Callers(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func target() string {
	return "target"
}

func caller1() string {
	return target()
}

func caller2() string {
	return target()
}

func main() {
	caller1()
}
`)
	writeFile(t, filepath.Join(dir, "other.go"), `
package main

func caller3() string {
	return target()
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.CallHierarchyTool()
	params := map[string]any{
		"function_name": "target",
		"direction":     "callers",
	}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	// caller output format: "file:line: call to target()"
	if !strings.Contains(text, "call to target()") {
		t.Errorf("expected 'call to target()' in output, got:\n%s", text)
	}
	// Should have 3 call sites
	if strings.Count(text, "call to target()") < 3 {
		t.Errorf("expected at least 3 call sites, got %d, output:\n%s",
			strings.Count(text, "call to target()"), text)
	}
}

func TestCallHierarchy_NotFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func main() {}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.CallHierarchyTool()
	params := map[string]any{
		"function_name": "NonExistent",
		"direction":     "callees",
	}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "not found") {
		t.Errorf("expected 'not found', got:\n%s", text)
	}
}

func TestCallHierarchy_MaxDepth(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func level3() string { return "deep" }
func level2() string { return level3() }
func level1() string { return level2() }
func main() { level1() }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	// max_depth=1: only direct callees of level1
	tool := tools.CallHierarchyTool()
	params := map[string]any{
		"function_name": "level1",
		"direction":     "callees",
		"max_depth":     1,
	}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "level2") {
		t.Errorf("level1→level2 should appear at depth=1, got:\n%s", text)
	}
	if strings.Contains(text, "level3") {
		t.Errorf("level3 should NOT appear at depth=1, got:\n%s", text)
	}
}

func TestCallHierarchy_EmptyFunctionName(t *testing.T) {
	tool := tools.CallHierarchyTool()
	params := map[string]any{"function_name": ""}
	_, err := tool.Execute(context.Background(), "call5", params, nil)
	if err == nil {
		t.Fatal("expected error for empty function_name")
	}
}

func TestCallHierarchy_BothDirections(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func callee() string { return "x" }
func target() string { return callee() }
func caller() { target() }
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.CallHierarchyTool()
	params := map[string]any{
		"function_name": "target",
		"direction":     "both",
	}
	res, err := tool.Execute(context.Background(), "call6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "CALLEES") {
		t.Error("expected CALLEES section")
	}
	if !strings.Contains(text, "CALLERS") {
		t.Error("expected CALLERS section")
	}
	if !strings.Contains(text, "callee") {
		t.Error("expected 'callee' in callees output")
	}
	if !strings.Contains(text, "call to target()") {
		t.Errorf("expected 'call to target()' in callers output, got:\n%s", text)
	}
}

func init() {
	_ = os.MkdirAll
}
