package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYAMLQueryTool_SimpleValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte("name: alice\nage: 30\n"), 0644)

	tool := YAMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "name",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "alice") {
		t.Errorf("expected alice, got %q", result.Content[0].Text)
	}
}

func TestYAMLQueryTool_NestedArray(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")
	os.WriteFile(path, []byte("servers:\n  - host: s1\n    port: 80\n  - host: s2\n    port: 443\n"), 0644)

	tool := YAMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "servers.1.host",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "s2") {
		t.Errorf("expected s2, got %q", result.Content[0].Text)
	}
}

func TestYAMLQueryTool_FileNotFound(t *testing.T) {
	tool := YAMLQueryTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": "/nonexistent/file.yaml",
	}, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
