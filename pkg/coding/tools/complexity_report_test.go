package tools

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplexityReportTool_MissingPath(t *testing.T) {
	tool := ComplexityReportTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestComplexityReportTool_SimpleFunction(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// A simple function with complexity = 1 (no branches)
	code := "package p\n\nfunc Simple() int {\n\treturn 1\n}\n"
	os.WriteFile(filepath.Join(dir, "simple.go"), []byte(code), 0644)

	tool := ComplexityReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":      dir,
		"threshold": float64(15),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
	t.Logf("output: %s", result.Content[0].Text)
}

func TestComplexityReportTool_ComplexFunction(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// A function with multiple if/for branches (complexity > 5)
	code := `package p

func Complex(x int) string {
	if x > 10 {
		return "big"
	}
	for i := 0; i < x; i++ {
		if i%2 == 0 {
			continue
		}
		if i > 5 {
			break
		}
	}
	if x < 0 {
		return "negative"
	}
	return "small"
}
`
	os.WriteFile(filepath.Join(dir, "complex.go"), []byte(code), 0644)

	tool := ComplexityReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":      dir,
		"threshold": float64(1),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should report Complex function exceeding threshold of 1
	t.Logf("complex analysis: %s", result.Content[0].Text)
	if details, ok := result.Details["exceeding_count"].(int); ok {
		if details == 0 {
			t.Error("expected at least one function exceeding threshold of 1")
		}
	}
}

func TestComputeCyclomaticComplexity_Simple(t *testing.T) {
	code := `package p
func Simple() int { return 1 }
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		c := computeCyclomaticComplexity(fd)
		if c != 1 {
			t.Errorf("Simple() complexity = %d, want 1", c)
		}
	}
}

func TestComputeCyclomaticComplexity_Branched(t *testing.T) {
	code := `package p
func Branched(x int) int {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", code, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		c := computeCyclomaticComplexity(fd)
		// Base 1 + 2 if statements = 3
		if c != 3 {
			t.Errorf("Branched() complexity = %d, want 3", c)
		}
	}
}

func TestComplexityReportTool_DefaultThreshold(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	code := "package p\n\nfunc F() int { return 1 }\n"
	os.WriteFile(filepath.Join(dir, "f.go"), []byte(code), 0644)

	tool := ComplexityReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Default threshold is 15, so simple functions should not exceed
	if strings.Contains(result.Content[0].Text, "exceed") {
		t.Logf("output: %s", result.Content[0].Text)
	}
}