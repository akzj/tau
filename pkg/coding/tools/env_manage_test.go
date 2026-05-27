package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestEnvManageTool_ListEmpty(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	params := map[string]any{"action": "list", "file": filepath.Join(dir, ".env")}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "no .env file") {
		t.Fatalf("expected '(no .env file)', got: %s", text)
	}
}

func TestEnvManageTool_SetAndList(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	envFile := filepath.Join(dir, ".env")

	// Set a value
	params := map[string]any{"action": "set", "key": "MY_KEY", "value": "my_value", "file": envFile}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("set: expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "Set MY_KEY=my_value") {
		t.Fatalf("set: expected confirmation, got: %s", res.Content[0].Text)
	}

	// List and verify
	params = map[string]any{"action": "list", "file": envFile}
	res, err = tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("list: expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "MY_KEY=my_value") {
		t.Fatalf("list: expected 'MY_KEY=my_value', got: %s", res.Content[0].Text)
	}
}

func TestEnvManageTool_Get(t *testing.T) {
	os.Setenv("TAU_TEST_VAR", "test123")
	defer os.Unsetenv("TAU_TEST_VAR")

	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	params := map[string]any{"action": "get", "key": "TAU_TEST_VAR"}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "TAU_TEST_VAR=test123") {
		t.Fatalf("expected 'TAU_TEST_VAR=test123', got: %s", res.Content[0].Text)
	}
}

func TestEnvManageTool_GetNotSet(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	params := map[string]any{"action": "get", "key": "NONEXISTENT_VAR_XYZ"}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "(not set)") {
		t.Fatalf("expected '(not set)', got: %s", res.Content[0].Text)
	}
}

func TestEnvManageTool_MissingKeyForGet(t *testing.T) {
	tool := tools.EnvManageTool()
	params := map[string]any{"action": "get"}
	_, err := tool.Execute(context.Background(), "call6", params, nil)
	if err == nil {
		t.Fatal("expected error for get without key")
	}
}

func TestEnvManageTool_MissingKeyValueForSet(t *testing.T) {
	tool := tools.EnvManageTool()
	params := map[string]any{"action": "set", "key": "K"}
	_, err := tool.Execute(context.Background(), "call7", params, nil)
	if err == nil {
		t.Fatal("expected error for set without value")
	}
}

func TestEnvManageTool_Delete(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	envFile := filepath.Join(dir, ".env")

	// Set then delete
	tool.Execute(context.Background(), "call8", map[string]any{"action": "set", "key": "DEL_KEY", "value": "val", "file": envFile}, nil)

	params := map[string]any{"action": "delete", "key": "DEL_KEY", "file": envFile}
	res, err := tool.Execute(context.Background(), "call9", params, nil)
	if err != nil {
		t.Fatalf("delete: expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "Deleted DEL_KEY") {
		t.Fatalf("expected 'Deleted DEL_KEY', got: %s", res.Content[0].Text)
	}

	// Verify deleted
	params = map[string]any{"action": "list", "file": envFile}
	res, _ = tool.Execute(context.Background(), "call10", params, nil)
	if strings.Contains(res.Content[0].Text, "DEL_KEY=val") {
		t.Fatal("expected DEL_KEY to be deleted")
	}
}

func TestEnvManageTool_UnknownAction(t *testing.T) {
	tool := tools.EnvManageTool()
	params := map[string]any{"action": "invalid"}
	_, err := tool.Execute(context.Background(), "call11", params, nil)
	if err == nil {
		t.Fatal("expected error for unknown action")
	}
}

func TestEnvManageTool_DefaultFile(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	tool := tools.EnvManageTool()
	params := map[string]any{"action": "list"}
	res, err := tool.Execute(context.Background(), "call12", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	// Should look at workspace/.env
	_ = res
}
