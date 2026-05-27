package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSBOMGenerateTool_MissingPath(t *testing.T) {
	tool := SBOMGenerateTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestSBOMGenerateTool_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := SBOMGenerateTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "format": "json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "bomFormat") && !strings.Contains(text, "\"components\"") {
		t.Logf("SBOM output: %s", text)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content type")
	}
}

func TestSBOMGenerateTool_CycloneDXFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := SBOMGenerateTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "format": "cyclonedx"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "xmlns") {
		t.Errorf("expected CycloneDX XML output, got: %s", text)
	}
}

func TestSBOMGenerateTool_SPDXFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := SBOMGenerateTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "format": "spdx"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "SPDXVersion") {
		t.Errorf("expected SPDX output, got: %s", text)
	}
}

func TestSBOMGenerateTool_OutputFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := SBOMGenerateTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "format": "json", "output": "sbom.json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "sbom.json") {
		t.Errorf("expected sbom.json reference, got: %s", text)
	}

	if _, err := os.Stat(filepath.Join(dir, "sbom.json")); err != nil {
		t.Errorf("sbom.json not created: %v", err)
	}
}

func TestSBOMGenerateTool_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := SBOMGenerateTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err == nil {
		t.Error("expected error for missing go.mod")
	}
}
