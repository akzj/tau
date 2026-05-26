package core

import "testing"

func TestVersionDefaults(t *testing.T) {
	if Version != "dev" {
		t.Errorf("expected 'dev', got %q", Version)
	}
	if BuildTime == "" {
		t.Error("BuildTime should not be empty")
	}
	if CommitSHA == "" {
		t.Error("CommitSHA should not be empty")
	}
}
