//go:build !no_plugins

package core

import (
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
