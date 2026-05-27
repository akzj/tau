package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseNotesTool_MissingFromTag(t *testing.T) {
	tool := ReleaseNotesTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing from_tag")
	}
}

func TestReleaseNotesTool_NoGit(t *testing.T) {
	// Without git in path this should fail
	tool := ReleaseNotesTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from_tag": "v1.0.0",
	}, nil)
	if err != nil {
		if strings.Contains(err.Error(), "git not available") {
			return // expected
		}
		if strings.Contains(err.Error(), "check tags exist") {
			return // tag doesn't exist
		}
	}
}

func TestReleaseNotesTool_WithTags(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	// Create a tag
	exec.Command("git", "-C", dir, "tag", "v1.0.0").Run()

	// Make another commit and tag
	os.WriteFile(filepath.Join(dir, "CHANGES.md"), []byte("# changes"), 0644)
	exec.Command("git", "-C", dir, "add", "CHANGES.md").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: add changes doc").Run()
	exec.Command("git", "-C", dir, "tag", "v1.1.0").Run()

	tool := ReleaseNotesTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from_tag": "v1.0.0",
		"to_tag":   "v1.1.0",
	}, nil)
	if err != nil {
		t.Fatalf("release notes should work: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Release Notes") {
		t.Errorf("expected 'Release Notes' in output, got: %s", text)
	}
}

func TestReleaseNotesTool_JSONFormat(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v2.0.0").Run()
	os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("new feature"), 0644)
	exec.Command("git", "-C", dir, "add", "feat.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "feat: new thing").Run()
	exec.Command("git", "-C", dir, "tag", "v2.1.0").Run()

	tool := ReleaseNotesTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from_tag": "v2.0.0",
		"to_tag":   "v2.1.0",
		"format":   "json",
	}, nil)
	if err != nil {
		t.Fatalf("release notes JSON: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "\"from\"") {
		t.Errorf("expected JSON output with 'from' key, got: %s", text)
	}
}

func TestReleaseNotesTool_FormatDefault(t *testing.T) {
	dir := setupGitRepo(t)
	WorkspaceRoot = dir

	exec.Command("git", "-C", dir, "tag", "v3.0.0").Run()
	os.WriteFile(filepath.Join(dir, "fix.txt"), []byte("fix bug"), 0644)
	exec.Command("git", "-C", dir, "add", "fix.txt").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "fix: resolve crash").Run()
	exec.Command("git", "-C", dir, "tag", "v3.1.0").Run()

	tool := ReleaseNotesTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{
		"from_tag": "v3.0.0",
		"to_tag":   "v3.1.0",
	}, nil)
	if err != nil {
		t.Fatalf("release notes default format: %v", err)
	}
	text := result.Content[0].Text
	if !strings.Contains(text, "Bug Fixes") {
		t.Errorf("expected 'Bug Fixes' category for fix commit, got: %s", text)
	}
}