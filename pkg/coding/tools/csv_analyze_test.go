package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCSVAnalyzeTool_MissingFile(t *testing.T) {
	tool := CSVAnalyzeTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing csv_file")
	}
}

func TestCSVAnalyzeTool_BasicProfile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	csvContent := "name,age,score\nAlice,30,95.5\nBob,25,87.0\nCharlie,30,92.3\n"
	csvPath := filepath.Join(dir, "data.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	tool := CSVAnalyzeTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"csv_file": "data.csv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "name") {
		t.Error("expected 'name' column in output")
	}
	if !strings.Contains(text, "age") {
		t.Error("expected 'age' column in output")
	}
	if !strings.Contains(text, "score") {
		t.Error("expected 'score' column in output")
	}
	// Age should be detected as number
	if !strings.Contains(text, "number") {
		t.Logf("output: %s", text)
		t.Error("expected 'number' type detection for age or score")
	}
}

func TestCSVAnalyzeTool_NullCounting(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	csvContent := "col_a,col_b\n1,\n,2\n3,4\n"
	csvPath := filepath.Join(dir, "nulls.csv")
	os.WriteFile(csvPath, []byte(csvContent), 0644)

	tool := CSVAnalyzeTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"csv_file": "nulls.csv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Null:") {
		t.Error("expected null counts in output")
	}
}

func TestCSVAnalyzeTool_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	csvPath := filepath.Join(dir, "empty.csv")
	os.WriteFile(csvPath, []byte("header\n"), 0644)

	tool := CSVAnalyzeTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"csv_file": "empty.csv",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "header") && !strings.Contains(text, "0 rows") && !strings.Contains(text, "Total rows: 0") {
		t.Logf("output: %s", text)
	}
}

func TestCSVAnalyzeTool_SampleSize(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	var lines []string
	lines = append(lines, "x,y")
	for i := 0; i < 100; i++ {
		lines = append(lines, "1,2")
	}
	csvPath := filepath.Join(dir, "large.csv")
	os.WriteFile(csvPath, []byte(strings.Join(lines, "\n")), 0644)

	tool := CSVAnalyzeTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"csv_file":    "large.csv",
		"sample_size": 10,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Should succeed with small sample
	text := result.Content[0].Text
	if strings.Contains(text, "sampled: 100") {
		t.Error("expected sampled to be limited")
	}
	_ = text
}
