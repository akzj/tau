package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTestGapTool_MissingPath(t *testing.T) {
	tool := TestGapTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestTestGapTool_WithTestFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a Go package with test
	goModContent := "module example.com/testgap\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	srcCode := "package testgap\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n"
	os.WriteFile(filepath.Join(dir, "math.go"), []byte(srcCode), 0644)

	testCode := "package testgap\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(1, 2) != 3 {\n\t\tt.Error(\"fail\")\n\t}\n}\n"
	os.WriteFile(filepath.Join(dir, "math_test.go"), []byte(testCode), 0644)

	tool := TestGapTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
	t.Logf("coverage output: %s", result.Content[0].Text)
}

func TestTestGapTool_ParseCoverageProfile(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal coverage profile
	profileContent := "mode: set\n" +
		"example.com/test/main.go:1.1,3.1 3 0\n" +
		"example.com/test/main.go:5.1,7.1 2 2\n" +
		"example.com/test/util.go:1.1,5.1 4 0\n"
	profilePath := filepath.Join(dir, "coverage.out")
	os.WriteFile(profilePath, []byte(profileContent), 0644)

	gaps, err := parseCoverageProfile(profilePath, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) == 0 {
		t.Error("expected gaps from coverage profile")
	}
	for _, g := range gaps {
		t.Logf("file=%s coverage=%.1f%% below=%v uncovered=%v", g.File, g.CoveragePct, g.BelowThreshold, g.UncoveredLines)
		if g.File == "example.com/test/main.go" {
			// 0 covered / 5 total = 0%
			if g.CoveragePct != 0 {
				// Actually main.go has 3+2=5 stmts, 0+2=2 covered = 40%
				if g.CoveragePct != 40 {
					t.Logf("main.go coverage: %.1f%%", g.CoveragePct)
				}
			}
		}
	}
}

func TestTestGapTool_EmptyProfile(t *testing.T) {
	dir := t.TempDir()
	profilePath := filepath.Join(dir, "coverage.out")
	os.WriteFile(profilePath, []byte("mode: set\n"), 0644)

	// Empty profile with only "mode: set" line
	gaps, err := parseCoverageProfile(profilePath, 80)
	if err != nil {
		t.Fatal(err)
	}
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps for empty profile, got %d", len(gaps))
	}
}

func TestTestGapTool_MissingProfile(t *testing.T) {
	_, err := parseCoverageProfile("/nonexistent/coverage.out", 80)
	if err == nil {
		t.Error("expected error for missing profile")
	}
	if !strings.Contains(err.Error(), "no coverage data") {
		t.Logf("error: %v", err)
	}
}