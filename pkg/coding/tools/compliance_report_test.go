package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComplianceReportTool_MissingPath(t *testing.T) {
	tool := ComplianceReportTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestComplianceReportTool_InvalidStandard(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := ComplianceReportTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "standard": "INVALID"}, nil)
	if err == nil {
		t.Error("expected error for invalid standard")
	}
}

func TestComplianceReportTool_SOC2(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"crypto/tls\"\n\nfunc main() {}\n"), 0644)

	tool := ComplianceReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "standard": "SOC2", "format": "markdown"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "SOC2 Compliance Report") {
		t.Errorf("expected SOC2 report, got: %s", text)
	}
}

func TestComplianceReportTool_GDPR(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644)

	tool := ComplianceReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "standard": "GDPR", "format": "markdown"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "GDPR Compliance Report") {
		t.Errorf("expected GDPR report, got: %s", text)
	}
}

func TestComplianceReportTool_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"crypto/tls\"\n"), 0644)

	tool := ComplianceReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "standard": "PCI", "format": "json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "\"standard\"") {
		t.Errorf("expected JSON output, got: %s", text)
	}
}

func TestComplianceReportTool_HIPAA(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"log/slog\"\n\nfunc main() {}\n"), 0644)

	tool := ComplianceReportTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "standard": "HIPAA", "format": "markdown"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "HIPAA Compliance Report") {
		t.Errorf("expected HIPAA report, got: %s", text)
	}
}

func TestIsValidStandard(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"SOC2", true},
		{"soc2", true},
		{"GDPR", true},
		{"HIPAA", true},
		{"PCI", true},
		{"NIST", false},
		{"ISO27001", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isValidStandard(tt.input)
		if got != tt.want {
			t.Errorf("isValidStandard(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestScanDirForStrings(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"crypto/tls\"\n"), 0644)

	if !scanDirForStrings(dir, []string{"crypto/tls"}) {
		t.Error("expected to find crypto/tls")
	}
	if scanDirForStrings(dir, []string{"nonexistent"}) {
		t.Error("should not find nonexistent")
	}
}
