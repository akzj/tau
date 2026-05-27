package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitTagTool_List(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitTagTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "list",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// List should succeed even with no tags
	if result.Content[0].Type != "text" {
		t.Errorf("expected text content")
	}
}

func TestGitTagTool_MissingAction(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitTagTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestGitTagTool_CreateAndList(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitTagTool()
	// Create a tag
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "create", "name": "v1.0.0", "message": "first release",
	}, nil)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	// List tags
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "list",
	}, nil)
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if !strings.Contains(result.Content[0].Text, "v1.0.0") {
		t.Errorf("expected tag v1.0.0 in output, got: %s", result.Content[0].Text)
	}
}

func TestGitTagTool_Delete(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	tool := GitTagTool()
	// Create then delete
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "create", "name": "v2.0.0",
	}, nil)
	if err != nil {
		t.Fatalf("create tag: %v", err)
	}

	_, err = tool.Execute(context.Background(), "id", map[string]any{
		"action": "delete", "name": "v2.0.0",
	}, nil)
	if err != nil {
		t.Fatalf("delete tag: %v", err)
	}
}

func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	exec.Command("git", "init", dir).Run()
	exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "test").Run()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	exec.Command("git", "-C", dir, "add", ".").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "init").Run()
	return dir
}
