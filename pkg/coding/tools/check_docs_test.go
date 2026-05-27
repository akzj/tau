package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDocsTool_MissingPath(t *testing.T) {
	tool := CheckDocsTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestCheckDocsTool_ExportedMissingDoc(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a Go file with undocumented exported function
	code := "package p\n\nfunc HasNoDoc() {\n}\n"
	os.WriteFile(filepath.Join(dir, "nodoc.go"), []byte(code), 0644)

	tool := CheckDocsTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
		"type": "exported",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
	if !strings.Contains(result.Content[0].Text, "HasNoDoc") {
		t.Errorf("expected 'HasNoDoc' in output, got: %s", result.Content[0].Text)
	}
}

func TestCheckDocsTool_ExportedWithDoc(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a Go file with documented exported function
	code := "package p\n\n// HasDoc does something.\nfunc HasDoc() {\n}\n"
	os.WriteFile(filepath.Join(dir, "hasdoc.go"), []byte(code), 0644)

	tool := CheckDocsTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
		"type": "exported",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should NOT report HasDoc
	if strings.Contains(result.Content[0].Text, "HasDoc") {
		t.Errorf("'HasDoc' should not be reported as missing docs, got: %s", result.Content[0].Text)
	}
}

func TestCheckDocsTool_UnexportedIgnored(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a Go file with undocumented unexported function (should be ignored)
	code := "package p\n\nfunc hiddenFunc() {\n}\n"
	os.WriteFile(filepath.Join(dir, "hidden.go"), []byte(code), 0644)

	tool := CheckDocsTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
		"type": "exported",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(result.Content[0].Text, "hiddenFunc") {
		t.Errorf("unexported 'hiddenFunc' should be ignored by exported check")
	}
}

func TestCheckDocsTool_PackageDoc(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// No doc.go, no package doc comment
	code := "package p\n\nfunc F(){}\n"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte(code), 0644)

	tool := CheckDocsTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
		"type": "package",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should report missing package doc
	t.Logf("package check output: %s", result.Content[0].Text)
}

func TestCheckDocsTool_FilePath(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Single file check
	code := "package p\n\n// NotExported is not exported\nfunc NotExported() {\n}\n"
	path := filepath.Join(dir, "single.go")
	os.WriteFile(path, []byte(code), 0644)

	tool := CheckDocsTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path,
		"type": "exported",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
}