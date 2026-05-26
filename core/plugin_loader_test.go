//go:build !no_plugins

package core

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestDiscoverEmptyDir(t *testing.T) {
	t.Setenv("TAU_PLUGIN_DIR", "/tmp/tau-test-nonexistent")
	if err := Discover(); err != nil {
		t.Logf("Discover returned error (expected for missing dir): %v", err)
	}
}

func TestExternalPluginName(t *testing.T) {
	ep := &ExternalPlugin{name: "test", version: "0.1"}
	if ep.Name() != "test" {
		t.Error("expected name test")
	}
	if ep.Version() != "0.1" {
		t.Error("expected version 0.1")
	}
}

func TestPluginWorkspaceDiag(t *testing.T) {
	pluginBin := "/tmp/plugin-workspace-diag"
	if _, err := os.Stat(pluginBin); os.IsNotExist(err) {
		t.Skip("plugin binary not built — run: go build -o /tmp/plugin-workspace-diag ./cmd/plugin-workspace-diag/")
	}

	ep, err := startPlugin(pluginBin)
	if err != nil {
		t.Fatalf("start plugin: %v", err)
	}
	defer ep.cmd.Process.Kill()

	if ep.Name() != "plugin-workspace-diag" {
		t.Errorf("expected plugin-workspace-diag, got %q", ep.Name())
	}

	tools := ep.Tools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "plugin-workspace-diag" {
		t.Errorf("expected plugin-workspace-diag tool, got %q", tools[0].Name)
	}

	result, err := tools[0].Execute(context.Background(), "c1", map[string]any{"depth": 1}, nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Total files") {
		t.Errorf("expected 'Total files' in output, got: %s", text[:min(len(text), 200)])
	}
	if !strings.Contains(text, "Go") {
		t.Logf("plugin diag output (first 500): %s", text[:min(len(text), 500)])
	}
}
