package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestJSONQueryExtract(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	jsonData := `{"name":"Alice","age":30,"address":{"city":"NYC"}}`
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(jsonData), 0644)

	tool := tools.JSONQueryTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.json", "query": "name"}, nil)
	if err != nil {
		t.Fatalf("json_query: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Alice") {
		t.Errorf("expected Alice, got: %s", result.Content[0].Text)
	}
}

func TestJSONQueryNested(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	jsonData := `{"user":{"profile":{"email":"test@test.com"}}}`
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(jsonData), 0644)

	tool := tools.JSONQueryTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.json", "query": "user.profile.email"}, nil)
	if err != nil {
		t.Fatalf("json_query: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "test@test.com") {
		t.Errorf("expected email, got: %s", result.Content[0].Text)
	}
}

func TestJSONQueryNotFound(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	jsonData := `{"a":1}`
	os.WriteFile(filepath.Join(dir, "data.json"), []byte(jsonData), 0644)

	tool := tools.JSONQueryTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.json", "query": "nonexistent"}, nil)
	if !strings.Contains(result.Content[0].Text, "not found") {
		t.Errorf("expected not found, got: %s", result.Content[0].Text)
	}
}
