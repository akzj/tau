package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditLicenseTool_MissingPath(t *testing.T) {
	tool := AuditLicenseTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestAuditLicenseTool_HeuristicFallback(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a minimal go.mod
	goModContent := "module example.com/test\n\ngo 1.21\n\nrequire github.com/example/foo v1.0.0\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	tool := AuditLicenseTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": dir,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
	if !strings.Contains(result.Content[0].Text, "unknown") && !strings.Contains(result.Content[0].Text, "github.com") {
		t.Logf("output: %s", result.Content[0].Text)
	}
}

func TestAuditLicenseTool_CustomAllowed(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goModContent := "module example.com/test\n\ngo 1.21\n\nrequire github.com/example/bar v2.0.0\n"
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goModContent), 0644)

	tool := AuditLicenseTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path":             dir,
		"allowed_licenses": "MIT,GPL-3.0",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
}

func TestParseAllowedLicenses(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 5}, // default: MIT, Apache-2.0, BSD-3-Clause, BSD-2-Clause, MPL-2.0
		{"MIT", 1},
		{"MIT,Apache-2.0", 2},
		{"MIT, Apache,   BSD", 3},
	}

	for _, tt := range tests {
		result := parseAllowedLicenses(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseAllowedLicenses(%q) = %d items, want %d", tt.input, len(result), tt.expected)
		}
	}
}

func TestIsLicenseAllowed(t *testing.T) {
	allowed := []string{"MIT", "Apache-2.0"}

	if !isLicenseAllowed("MIT", allowed) {
		t.Error("MIT should be allowed")
	}
	if !isLicenseAllowed("mit", allowed) {
		t.Error("mit (lowercase) should be allowed (case insensitive)")
	}
	if isLicenseAllowed("GPL-3.0", allowed) {
		t.Error("GPL-3.0 should NOT be allowed")
	}
	if !isLicenseAllowed("Apache-2.0", allowed) {
		t.Error("Apache-2.0 should be allowed")
	}
}

func TestIdentifyLicense(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"Permission is hereby granted, free of charge, to any person obtaining a copy", "MIT"},
		{"Licensed under the Apache License, Version 2.0", "Apache-2.0"},
		{"GNU General Public License Version 3", "GPL-3.0"},
		{"some random text without license info", "unknown"},
	}

	for _, tt := range tests {
		result := identifyLicense(tt.text)
		if result != tt.expected {
			t.Errorf("identifyLicense(%q) = %q, want %q", tt.text, result, tt.expected)
		}
	}
}