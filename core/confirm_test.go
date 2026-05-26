package core

import (
	"testing"
)

func TestIsDangerous(t *testing.T) {
	dangerous := []string{"write", "edit", "bash", "write_file", "exec_command", "web_fetch", "spawn_agent"}
	for _, name := range dangerous {
		if !IsDangerous(name) {
			t.Errorf("%s should be dangerous", name)
		}
	}
}

func TestIsSafe(t *testing.T) {
	safe := []string{"read", "grep", "glob", "list_files", "workspace_diag", "search_code"}
	for _, name := range safe {
		if IsDangerous(name) {
			t.Errorf("%s should be safe", name)
		}
	}
}

func TestConfirmYesMode(t *testing.T) {
	cfg := ConfirmConfig{Yes: true}
	if !Confirm(cfg, "write", nil) {
		t.Error("--yes should always confirm")
	}
}

func TestConfirmNonInteractive(t *testing.T) {
	cfg := ConfirmConfig{NonInteractive: true}
	if Confirm(cfg, "write", nil) {
		t.Error("non-interactive should auto-deny")
	}
}
