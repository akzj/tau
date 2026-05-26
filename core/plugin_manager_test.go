//go:build !no_plugins

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPluginInstallListRemove(t *testing.T) {
	dir := t.TempDir()

	pluginBin := "/tmp/plugin-example"
	if _, err := os.Stat(pluginBin); os.IsNotExist(err) {
		t.Skip("plugin-example not built — run: go build -o /tmp/plugin-example ./cmd/plugin-example/")
	}

	if err := InstallPlugin(pluginBin, dir); err != nil {
		t.Fatalf("install: %v", err)
	}

	infos, err := ListPlugins(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(infos) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(infos))
	}
	if infos[0].Name != "plugin-example" {
		t.Errorf("expected plugin-example, got %q", infos[0].Name)
	}
	if infos[0].Status != "ok" {
		t.Errorf("expected status 'ok', got %q", infos[0].Status)
	}
	if len(infos[0].Tools) == 0 {
		t.Error("expected at least 1 tool")
	}

	dstName := filepath.Base(pluginBin)
	if err := RemovePlugin(dstName, dir); err != nil {
		t.Fatalf("remove: %v", err)
	}

	infos, _ = ListPlugins(dir)
	if len(infos) != 0 {
		t.Errorf("expected 0 plugins after remove, got %d", len(infos))
	}
}

func TestPluginInstallNotFound(t *testing.T) {
	err := InstallPlugin("/nonexistent/path", t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent source")
	}
}

func TestPluginRemoveNotFound(t *testing.T) {
	err := RemovePlugin("nonexistent", t.TempDir())
	if err == nil {
		t.Error("expected error for nonexistent plugin")
	}
}
