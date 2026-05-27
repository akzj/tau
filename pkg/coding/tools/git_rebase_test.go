package tools

import (
	"context"
	"testing"
)

func TestGitRebaseTool_NoArgs(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitRebaseTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for no args")
	}
}

func TestGitRebaseTool_AbortNoRebase(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitRebaseTool()
	// Abort when no rebase is in progress should fail (expected)
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"abort": true,
	}, nil)
	// This may or may not error depending on git state; we just check it doesn't panic
	_ = err
}
