package core

import (
	"strings"
	"testing"
)

func TestGenerateBashCompletion(t *testing.T) {
	script, err := GenerateCompletion(ShellBash)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "complete -F _tau_completion tau") {
		t.Error("expected bash complete command")
	}
}

func TestGenerateZshCompletion(t *testing.T) {
	script, err := GenerateCompletion(ShellZsh)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "#compdef tau") {
		t.Error("expected zsh compdef")
	}
}

func TestGenerateFishCompletion(t *testing.T) {
	script, err := GenerateCompletion(ShellFish)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "complete -c tau") {
		t.Error("expected fish complete command")
	}
}

func TestGenerateInvalidShell(t *testing.T) {
	_, err := GenerateCompletion("powershell")
	if err == nil {
		t.Error("expected error for invalid shell")
	}
}
