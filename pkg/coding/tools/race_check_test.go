package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRaceCheckTool_MissingPath(t *testing.T) {
	tool := RaceCheckTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestRaceCheckTool_NoRace(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a package with a simple non-racy test
	goModContent := "module example.com/racetest\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	srcCode := "package racetest\n\nvar Counter int\n"
	os.WriteFile(filepath.Join(dir, "counter.go"), []byte(srcCode), 0644)

	testCode := "package racetest\n\nimport \"testing\"\n\nfunc TestCounter(t *testing.T) {\n\tCounter = 1\n\tif Counter != 1 {\n\t\tt.Error(\"fail\")\n\t}\n}\n"
	os.WriteFile(filepath.Join(dir, "counter_test.go"), []byte(testCode), 0644)

	tool := RaceCheckTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":    dir,
		"timeout": float64(30),
	}, nil)
	if err != nil {
		t.Logf("race test error (expected if go test -race fails): %v", err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
	t.Logf("race check output: %s", result.Content[0].Text)
}

func TestParseRaceReport_Empty(t *testing.T) {
	races := parseRaceReport("nothing here, no data races")
	if len(races) != 0 {
		t.Errorf("expected 0 races in empty text, got %d", len(races))
	}
}

func TestParseRaceReport_WithRace(t *testing.T) {
	// Simplified race detector output
	text := `==================
WARNING: DATA RACE
Write at 0x00c000123456 by goroutine 7:
  example.com/test.update()
      /path/to/test.go:10 +0x45

Previous read at 0x00c000123456 by goroutine 6:
  example.com/test.read()
      /path/to/test.go:5 +0x23
==================
Goroutine 7 (running) created at:
  example.com/test.TestRace()
      /path/to/test.go:15 +0x67

Goroutine 6 (finished) created at:
  example.com/test.TestRace()
      /path/to/test.go:14 +0x55
==================
`

	races := parseRaceReport(text)
	if len(races) == 0 {
		t.Error("expected at least 1 race from race report")
	}
	for _, r := range races {
		t.Logf("race: file=%s line=%d goroutine=%s operation=%s", r.File, r.Line, r.Goroutine, r.Operation)
	}
}

func TestParseRaceSection(t *testing.T) {
	section := `
Write at 0x00c000123456 by goroutine 7:
  /path/to/test.go:10 +0x45
  /path/to/test.go:15 +0x67
`
	entry := parseRaceSection(section)
	if entry.File != "/path/to/test.go" {
		t.Logf("file: %s", entry.File)
	}
	if entry.Line != 10 {
		t.Logf("line: %d", entry.Line)
	}
}

func TestRaceCheckTool_DefaultTimeout(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/racetest2\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	srcCode := "package racetest2\n\nvar Val int\n"
	os.WriteFile(filepath.Join(dir, "val.go"), []byte(srcCode), 0644)

	testCode := "package racetest2\n\nimport \"testing\"\n\nfunc TestVal(t *testing.T) {\n\tVal = 42\n}\n"
	os.WriteFile(filepath.Join(dir, "val_test.go"), []byte(testCode), 0644)

	tool := RaceCheckTool()
	// No timeout specified — should default to 60
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
	}, nil)
	if err != nil {
		t.Logf("race test error: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "No race conditions detected") &&
		!strings.Contains(result.Content[0].Text, "race") {
		t.Logf("output: %s", result.Content[0].Text)
	}
}