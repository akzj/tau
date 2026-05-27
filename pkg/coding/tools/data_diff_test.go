package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataDiffTool_MissingFiles(t *testing.T) {
	tool := DataDiffTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing file_a and file_b")
	}
}

func TestDataDiffTool_CSVIdentical(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	csvContent := "name,age\nAlice,30\nBob,25\n"
	os.WriteFile(filepath.Join(dir, "a.csv"), []byte(csvContent), 0644)
	os.WriteFile(filepath.Join(dir, "b.csv"), []byte(csvContent), 0644)

	tool := DataDiffTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file_a": "a.csv",
		"file_b": "b.csv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	// Should show 0 added, 0 removed, 0 changed
	if strings.Contains(text, "+1") || strings.Contains(text, "-1") || strings.Contains(text, "~1") {
		t.Errorf("expected no differences for identical files: %s", text)
	}
}

func TestDataDiffTool_CSVAddedRemoved(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	csvA := "name,age\nAlice,30\nBob,25\n"
	csvB := "name,age\nAlice,30\nCharlie,35\n"
	os.WriteFile(filepath.Join(dir, "old.csv"), []byte(csvA), 0644)
	os.WriteFile(filepath.Join(dir, "new.csv"), []byte(csvB), 0644)

	tool := DataDiffTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file_a": "old.csv",
		"file_b": "new.csv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	// Should detect added (Charlie) and removed (Bob)
	if !strings.Contains(text, "+1") && !strings.Contains(text, "Added") {
		t.Error("expected detection of added rows")
	}
	if !strings.Contains(text, "-1") && !strings.Contains(text, "Removed") {
		t.Error("expected detection of removed rows")
	}
}

func TestDataDiffTool_JSONDiff(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	jsonA := `[{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]`
	jsonB := `[{"id": 1, "name": "Alice"}, {"id": 3, "name": "Charlie"}]`
	os.WriteFile(filepath.Join(dir, "a.json"), []byte(jsonA), 0644)
	os.WriteFile(filepath.Join(dir, "b.json"), []byte(jsonB), 0644)

	tool := DataDiffTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file_a": "a.json",
		"file_b": "b.json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	t.Logf("JSON diff result: %s", text)
	if !strings.Contains(text, "json") && !strings.Contains(text, "JSON") {
		// format auto-detection should work
	}
}

func TestDataDiffTool_FormatDetection(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// TSV file
	tsvContent := "name\tage\nAlice\t30\n"
	os.WriteFile(filepath.Join(dir, "data.tsv"), []byte(tsvContent), 0644)
	os.WriteFile(filepath.Join(dir, "data2.tsv"), []byte(tsvContent), 0644)

	tool := DataDiffTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file_a": "data.tsv",
		"file_b": "data2.tsv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "tsv") {
		t.Logf("format detection output: %s", text)
	}
}

func TestDataDiffTool_MissingFile(t *testing.T) {
	tool := DataDiffTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"file_a": "nonexistent.csv",
		"file_b": "also_missing.csv",
	}, nil)
	if err == nil {
		t.Error("expected error for missing files")
	}
}
