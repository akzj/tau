package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestXMLQueryTool_SimpleTag(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xml")
	os.WriteFile(path, []byte("<root><name>alice</name><age>30</age></root>"), 0644)

	tool := XMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "root.name._text",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "alice") {
		t.Errorf("expected alice, got %q", result.Content[0].Text)
	}
}

func TestXMLQueryTool_WithAttrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xml")
	os.WriteFile(path, []byte(`<root><server host="s1" port="80"/></root>`), 0644)

	tool := XMLQueryTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": path, "query": "root.server",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "s1") {
		t.Errorf("expected s1 in result, got %q", result.Content[0].Text)
	}
}

func TestXMLQueryTool_FileNotFound(t *testing.T) {
	tool := XMLQueryTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"file": "/nonexistent/file.xml",
	}, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
