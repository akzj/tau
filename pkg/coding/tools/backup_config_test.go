package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupConfigTool_MissingPath(t *testing.T) {
	tool := BackupConfigTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestBackupConfigTool_MissingAction(t *testing.T) {
	tool := BackupConfigTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": ".",
	}, nil)
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestBackupConfigTool_BackupAndList(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a config file
	configContent := "PORT=8080\nHOST=localhost\n"
	configPath := filepath.Join(dir, ".env")
	os.WriteFile(configPath, []byte(configContent), 0644)

	// Create backup dir
	backupDir := filepath.Join(dir, "backups")
	_ = backupDir

	tool := BackupConfigTool()

	// Backup
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action":     "backup",
		"path":       ".env",
		"backup_dir": "backups",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Backup Created") {
		t.Errorf("expected 'Backup Created' in output: %s", text)
	}

	// Verify backup exists
	entries, _ := os.ReadDir(backupDir)
	if len(entries) == 0 {
		t.Error("no backup created")
	}

	// List backups
	result2, err := tool.Execute(context.Background(), "id", map[string]any{
		"action":     "list",
		"path":       ".",
		"backup_dir": "backups",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text2 := result2.Content[0].Text
	if !strings.Contains(text2, "Backup List") {
		t.Errorf("expected 'Backup List' in output: %s", text2)
	}
}

func TestBackupConfigTool_Restore(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create original config
	configPath := filepath.Join(dir, "config.yaml")
	os.WriteFile(configPath, []byte("key: original_value\n"), 0644)

	backupDir := filepath.Join(dir, "backups")
	_ = backupDir

	tool := BackupConfigTool()

	// Backup
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action":     "backup",
		"path":       "config.yaml",
		"backup_dir": "backups",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Modify original
	os.WriteFile(configPath, []byte("key: modified_value\n"), 0644)

	// Restore
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action":     "restore",
		"path":       "config.yaml",
		"backup_dir": "backups",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Restore Complete") {
		t.Errorf("expected 'Restore Complete': %s", text)
	}

	// Verify restored content
	data, _ := os.ReadFile(configPath)
	if strings.Contains(string(data), "modified") {
		t.Error("file was not restored to original content")
	}
}

func TestBackupConfigTool_NonConfigFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a non-config file
	os.WriteFile(filepath.Join(dir, "script.sh"), []byte("echo hello"), 0644)

	tool := BackupConfigTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "backup",
		"path":   "script.sh",
	}, nil)
	if err == nil {
		text := result.Content[0].Text
		// Should bail gracefully
		t.Logf("non-config result: %s", text)
	}
}

func TestBackupConfigTool_UnknownAction(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := BackupConfigTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "invalid",
		"path":   ".",
	}, nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}
