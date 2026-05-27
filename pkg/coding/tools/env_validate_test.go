package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestEnvValidateCheckDeps(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvValidateTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "check-deps"}, nil)
	if err != nil {
		t.Fatalf("env_validate: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Dependency Check") {
		t.Errorf("expected 'Dependency Check', got: %s", text)
	}
}

func TestEnvValidateGoMod(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.21\n"), 0644)

	tool := tools.EnvValidateTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "go"}, nil)
	if err != nil {
		t.Fatalf("env_validate go: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Go Mod Verify") {
		t.Errorf("expected 'Go Mod Verify', got: %s", text)
	}
}

func TestEnvValidateConfig(t *testing.T) {
	origRoot := tools.WorkspaceRoot
	defer func() { tools.WorkspaceRoot = origRoot }()

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"key":"value"}`), 0644)

	tool := tools.EnvValidateTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "check-config"}, nil)
	if err != nil {
		t.Fatalf("env_validate config: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "valid JSON") {
		t.Errorf("expected valid JSON, got: %s", text)
	}
}
