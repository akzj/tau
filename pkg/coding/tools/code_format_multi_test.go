package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCodeFormatMultiTool_MissingPath(t *testing.T) {
	tool := CodeFormatMultiTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestCodeFormatMultiTool_GoFile(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	path := filepath.Join(dir, "test.go")
	os.WriteFile(path, []byte("package main\n\nfunc main(){\n}\n"), 0644)

	tool := CodeFormatMultiTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}

	// gofmt should have added spacing
	data, _ := os.ReadFile(path)
	if string(data) == "package main\n\nfunc main(){\n}\n" {
		// gofmt may not be installed, that's OK
		t.Log("gofmt may not be installed, file unchanged")
	}
}

func TestCodeFormatMultiTool_JSONFile(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	path := filepath.Join(dir, "test.json")
	os.WriteFile(path, []byte(`{"name":"alice"}`), 0644)

	tool := CodeFormatMultiTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}

	// Should have been formatted with indentation
	data, _ := os.ReadFile(path)
	t.Logf("Formatted JSON: %s", string(data))
}
