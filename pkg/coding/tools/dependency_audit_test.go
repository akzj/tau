package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDependencyAuditTool_MissingPath(t *testing.T) {
	tool := DependencyAuditTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestDependencyAuditTool_WithGoMod(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := DependencyAuditTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Audited") {
		t.Errorf("expected audit summary, got: %s", text)
	}
}

func TestDependencyAuditTool_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := DependencyAuditTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err == nil {
		t.Error("expected error for missing go.mod")
	}
}

func TestDependencyAuditTool_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := DependencyAuditTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "output": "json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content type")
	}
}

func TestVersionLE(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"1.0.0", "2.0.0", true},
		{"2.0.0", "1.0.0", false},
		{"1.0.0", "1.0.0", true},
		{"0.1.0", "0.2.0", true},
		{"v1.0.0", "v2.0.0", true},
		{"", "1.0.0", true},
	}
	for _, tt := range tests {
		got := versionLE(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("versionLE(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestListModules_Fallback(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	mods, err := listModules(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(mods) == 0 {
		t.Error("expected at least 1 module (self)")
	}
	foundSelf := false
	for _, m := range mods {
		if m.Path == "example.com/test" {
			foundSelf = true
		}
	}
	if !foundSelf {
		t.Error("expected self module in list")
	}
}
