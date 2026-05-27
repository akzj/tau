package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestRefactorRename_Function(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func oldFunc() string {
	return "hello"
}

func callerFunc() string {
	return oldFunc()
}

func main() {
	oldFunc()
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.RefactorRenameTool()
	params := map[string]any{
		"old_name": "oldFunc",
		"new_name": "newFunc",
		"dry_run":  false,
	}
	res, err := tool.Execute(context.Background(), "rr1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "oldFunc → newFunc") {
		t.Errorf("expected rename summary, got:\n%s", text)
	}
	if !strings.Contains(text, "Files changed: 1") {
		t.Errorf("expected 1 file changed, got:\n%s", text)
	}
	if !strings.Contains(text, "Replacements: 3") {
		t.Errorf("expected 3 replacements (decl + 2 calls), got:\n%s", text)
	}

	// Verify the file was actually modified
	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if strings.Contains(str, "oldFunc") {
		t.Errorf("oldFunc still present in modified file:\n%s", str)
	}
	if !strings.Contains(str, "newFunc") {
		t.Errorf("newFunc not found in modified file:\n%s", str)
	}
}

func TestRefactorRename_Variable(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func compute() int {
	oldVar := 42
	return oldVar * 2
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.RefactorRenameTool()
	params := map[string]any{
		"old_name": "oldVar",
		"new_name": "newVar",
		"dry_run":  false,
	}
	res, err := tool.Execute(context.Background(), "rr2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "oldVar → newVar") {
		t.Errorf("expected rename summary, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if strings.Contains(str, "oldVar") {
		t.Errorf("oldVar still present:\n%s", str)
	}
	if !strings.Contains(str, "newVar") {
		t.Errorf("newVar not found:\n%s", str)
	}
}

func TestRefactorRename_Type(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

type oldType struct {
	value int
}

func newMethod(t oldType) int {
	return t.value
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.RefactorRenameTool()
	params := map[string]any{
		"old_name": "oldType",
		"new_name": "newType",
		"dry_run":  false,
	}
	res, err := tool.Execute(context.Background(), "rr3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "oldType → newType") {
		t.Errorf("expected rename summary, got:\n%s", text)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	str := string(content)
	if strings.Contains(str, "oldType") {
		t.Errorf("oldType still present:\n%s", str)
	}
	if !strings.Contains(str, "newType") {
		t.Errorf("newType not found:\n%s", str)
	}
}

func TestRefactorRename_DryRun(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func dryFunc() string {
	return "dry"
}

func main() {
	dryFunc()
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	original, _ := os.ReadFile(filepath.Join(dir, "main.go"))

	tool := tools.RefactorRenameTool()
	params := map[string]any{
		"old_name": "dryFunc",
		"new_name": "wetFunc",
		"dry_run":  true,
	}
	res, err := tool.Execute(context.Background(), "rr4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "DRY RUN") {
		t.Errorf("expected DRY RUN marker, got:\n%s", text)
	}

	// File should be unchanged
	current, _ := os.ReadFile(filepath.Join(dir, "main.go"))
	if string(current) != string(original) {
		t.Errorf("file was modified despite dry_run=true")
	}
}

func TestRefactorRename_InvalidIdentifiers(t *testing.T) {
	tool := tools.RefactorRenameTool()

	// Invalid old_name
	params := map[string]any{"old_name": "123invalid", "new_name": "valid"}
	_, err := tool.Execute(context.Background(), "rr5", params, nil)
	if err == nil || !strings.Contains(err.Error(), "valid Go identifier") {
		t.Errorf("expected identifier validation error, got: %v", err)
	}

	// Invalid new_name
	params = map[string]any{"old_name": "valid", "new_name": "123invalid"}
	_, err = tool.Execute(context.Background(), "rr6", params, nil)
	if err == nil || !strings.Contains(err.Error(), "valid Go identifier") {
		t.Errorf("expected identifier validation error, got: %v", err)
	}
}

func TestRefactorRename_MissingRequired(t *testing.T) {
	tool := tools.RefactorRenameTool()

	_, err := tool.Execute(context.Background(), "rr7", map[string]any{"old_name": ""}, nil)
	if err == nil {
		t.Fatal("expected error for missing old_name")
	}
	_, err = tool.Execute(context.Background(), "rr8", map[string]any{"old_name": "a", "new_name": ""}, nil)
	if err == nil {
		t.Fatal("expected error for missing new_name")
	}
}

func TestRefactorRename_MultiFile(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "a.go"), `
package main

func multiFunc() string {
	return "multi"
}
`)
	writeFile(t, filepath.Join(dir, "b.go"), `
package main

func callerA() string {
	return multiFunc()
}
`)
	writeFile(t, filepath.Join(dir, "c.go"), `
package main

func callerB() {
	multiFunc()
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.RefactorRenameTool()
	params := map[string]any{
		"old_name": "multiFunc",
		"new_name": "renamedFunc",
		"dry_run":  false,
	}
	res, err := tool.Execute(context.Background(), "rr9", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Files changed: 3") {
		t.Errorf("expected 3 files changed, got:\n%s", text)
	}

	// Verify all files are clean
	for _, f := range []string{"a.go", "b.go", "c.go"} {
		content, _ := os.ReadFile(filepath.Join(dir, f))
		if strings.Contains(string(content), "multiFunc") {
			t.Errorf("%s still contains old name", f)
		}
	}
}
