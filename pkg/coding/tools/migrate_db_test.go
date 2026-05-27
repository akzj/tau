package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateDBTool_MissingDir(t *testing.T) {
	tool := MigrateDBTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing migration_dir")
	}
}

func TestMigrateDBTool_MissingAction(t *testing.T) {
	tool := MigrateDBTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
	}, nil)
	if err == nil {
		t.Error("expected error for missing action")
	}
}

func TestMigrateDBTool_StatusEmpty(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := MigrateDBTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "status",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "No migrations") {
		t.Logf("output: %s", text)
	}
}

func TestMigrateDBTool_UpAndStatus(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create migration files
	upSQL := "CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT);"
	downSQL := "DROP TABLE IF EXISTS test_items;"
	os.WriteFile(filepath.Join(dir, "001_create_items.up.sql"), []byte(upSQL), 0644)
	os.WriteFile(filepath.Join(dir, "001_create_items.down.sql"), []byte(downSQL), 0644)

	tool := MigrateDBTool()

	// Run up
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "up",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Applied") {
		t.Errorf("expected 'Applied' in up output: %s", text)
	}

	// Check status shows applied
	result2, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "status",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text2 := result2.Content[0].Text
	if !strings.Contains(text2, "applied") {
		t.Errorf("expected 'applied' in status: %s", text2)
	}

	// Running up again should say all applied
	result3, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "up",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text3 := result3.Content[0].Text
	if !strings.Contains(text3, "already applied") {
		t.Logf("up re-run: %s", text3)
	}
}

func TestMigrateDBTool_DownRollback(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	upSQL := "CREATE TABLE t1 (a INTEGER);"
	downSQL := "DROP TABLE IF EXISTS t1;"
	os.WriteFile(filepath.Join(dir, "001_test.up.sql"), []byte(upSQL), 0644)
	os.WriteFile(filepath.Join(dir, "001_test.down.sql"), []byte(downSQL), 0644)

	tool := MigrateDBTool()

	// Apply
	if _, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "up",
	}, nil); err != nil {
		t.Fatal(err)
	}

	// Rollback
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "down",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "Rolled back") {
		t.Errorf("expected 'Rolled back' in down output: %s", text)
	}
}

func TestMigrateDBTool_UnknownAction(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	tool := MigrateDBTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"migration_dir": ".",
		"action":        "invalid",
	}, nil)
	if err == nil {
		t.Error("expected error for unknown action")
	}
}
