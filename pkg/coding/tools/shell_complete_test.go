package tools

import (
	"context"
	"strings"
	"testing"
)

func TestShellCompleteTool_Bash(t *testing.T) {
	tool := ShellCompleteTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"shell": "bash",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "_tau_completion") {
		t.Errorf("expected bash completion script, got: %s", result.Content[0].Text[:100])
	}
}

func TestShellCompleteTool_Zsh(t *testing.T) {
	tool := ShellCompleteTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"shell": "zsh",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "#compdef tau") {
		t.Errorf("expected zsh completion script")
	}
}

func TestShellCompleteTool_Fish(t *testing.T) {
	tool := ShellCompleteTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"shell": "fish",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.Content[0].Text, "complete -c tau") {
		t.Errorf("expected fish completion script")
	}
}

func TestShellCompleteTool_InvalidShell(t *testing.T) {
	tool := ShellCompleteTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"shell": "invalid",
	}, nil)
	if err == nil {
		t.Error("expected error for invalid shell")
	}
}
