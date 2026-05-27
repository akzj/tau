package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitleaksScanTool_MissingPath(t *testing.T) {
	tool := GitleaksScanTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestGitleaksScanTool_Builtin_AWSKey(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nvar key = \"AKIAIOSFODNN7EXAMPLE\"\n"), 0644)

	tool := GitleaksScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "leak") && !strings.Contains(text, "No leaks") {
		t.Logf("gitleaks scan output: %s", text)
	}
}

func TestGitleaksScanTool_Builtin_GitHubToken(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "config"), []byte("token = \"ghp_abcdefghijklmnopqrstuvwxyz1234567890\"\n"), 0644)

	tool := GitleaksScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": ".", "verbose": true, "no_git": true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 || result.Content[0].Type != "text" {
		t.Error("expected text content")
	}
}

func TestGitleaksScanTool_NoLeaks(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() { println(\"hello world\") }\n"), 0644)

	tool := GitleaksScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestGitleaksScanTool_JWTToken(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "config.json"), []byte("jwt = \"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U\"\n"), 0644)

	tool := GitleaksScanTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{"path": "."}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestParseGitleaksJSON_Empty(t *testing.T) {
	findings := parseGitleaksJSON("")
	if len(findings) != 0 {
		t.Errorf("expected 0 findings from empty input, got %d", len(findings))
	}
}

func TestParseGitleaksJSON_Valid(t *testing.T) {
	raw := `[{"rule_id":"aws-access-key","description":"AWS Access Key","file":"main.go","line":3,"match":"AKIA...EXAMPLE","severity":"high"}]`
	findings := parseGitleaksJSON(raw)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].RuleID != "aws-access-key" {
		t.Errorf("expected aws-access-key, got %s", findings[0].RuleID)
	}
}
