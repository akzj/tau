package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLicenseHeaderTool_MissingPath(t *testing.T) {
	tool := LicenseHeaderTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestLicenseHeaderTool_MissingHeaderText(t *testing.T) {
	tool := LicenseHeaderTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err == nil {
		t.Error("expected error for missing license_header_text")
	}
}

func TestLicenseHeaderTool_CheckMode(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	header := "Copyright 2024 Example Inc.\nLicensed under MIT"

	tool := LicenseHeaderTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":                ".",
		"license_header_text": header,
		"action":              "check",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Files checked") {
		t.Errorf("expected check output, got: %s", text)
	}
	if !strings.Contains(text, "Missing: 1") {
		t.Errorf("expected 1 missing, got: %s", text)
	}
}

func TestLicenseHeaderTool_HeaderPresent(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	header := "Copyright 2024 Example Inc.\nLicensed under MIT"
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("// Copyright 2024 Example Inc.\n// Licensed under MIT\n\npackage main\n\nfunc main() {}\n"), 0644)

	tool := LicenseHeaderTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":                ".",
		"license_header_text": header,
		"action":              "check",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Correct: 1") {
		t.Errorf("expected 1 correct, got: %s", text)
	}
}

func TestLicenseHeaderTool_FixMode(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	header := "Copyright 2024 Example Inc.\nLicensed under MIT"

	tool := LicenseHeaderTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":                ".",
		"license_header_text": header,
		"action":              "fix",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Fixed: 1") {
		t.Errorf("expected 1 fixed, got: %s", text)
	}

	modified, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(modified), "Copyright") {
		t.Errorf("file should contain copyright header, got: %s", string(modified))
	}
}

func TestLicenseHeaderTool_WithShebang(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "script.py"), []byte("#!/usr/bin/env python\n\nprint('hello')\n"), 0644)

	header := "Copyright 2024 Example Inc.\nLicensed under MIT"

	tool := LicenseHeaderTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":                ".",
		"license_header_text": header,
		"action":              "fix",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Fixed: 1") {
		t.Errorf("expected 1 fixed for shebang file, got: %s", text)
	}

	modified, err := os.ReadFile(filepath.Join(dir, "script.py"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(modified), "#!/usr") {
		t.Errorf("shebang should be first, got: %s", string(modified))
	}
	if !strings.Contains(string(modified), "Copyright") {
		t.Errorf("file should contain copyright header, got: %s", string(modified))
	}
}
