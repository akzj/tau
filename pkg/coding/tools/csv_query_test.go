package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestCSVQueryCount(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	csvData := "name,age,city\nAlice,30,NYC\nBob,25,LA\n"
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte(csvData), 0644)

	tool := tools.CSVQueryTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.csv", "action": "count"}, nil)
	if err != nil {
		t.Fatalf("csv_query: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "2 rows") {
		t.Errorf("expected 2 rows, got: %s", result.Content[0].Text)
	}
}

func TestCSVQueryColumns(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	csvData := "name,age,city\nAlice,30,NYC\n"
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte(csvData), 0644)

	tool := tools.CSVQueryTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.csv", "action": "columns"}, nil)
	if err != nil {
		t.Fatalf("csv_query: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "name") || !strings.Contains(text, "age") || !strings.Contains(text, "city") {
		t.Errorf("expected all columns, got: %s", text)
	}
}

func TestCSVQueryFilter(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	csvData := "name,age,city\nAlice,30,NYC\nBob,25,LA\nAlice,28,SF\n"
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte(csvData), 0644)

	tool := tools.CSVQueryTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"file": "data.csv", "action": "filter", "column": "name", "value": "Alice"}, nil)
	if err != nil {
		t.Fatalf("csv_query: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "Matched: 2") {
		t.Errorf("expected Matched: 2, got: %s", result.Content[0].Text)
	}
}
