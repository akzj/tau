package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTOMLQueryTool_SimpleValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	os.WriteFile(path, []byte("title = \"My App\"\nversion = \"1.0.0\"\n"), 0644)

	tool := TOMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "title",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "My App") {
		t.Errorf("expected 'My App', got %q", result.Content[0].Text)
	}
}

func TestTOMLQueryTool_NestedValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.toml")
	os.WriteFile(path, []byte("[database]\nhost = \"localhost\"\nport = 5432\n"), 0644)

	tool := TOMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "database.port",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "5432") {
		t.Errorf("expected 5432, got %q", result.Content[0].Text)
	}
}

func TestTOMLQueryTool_FileNotFound(t *testing.T) {
	tool := TOMLQueryTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": "/nonexistent/file.toml",
	}, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
