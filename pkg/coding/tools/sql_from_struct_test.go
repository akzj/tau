package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSQLFromStructTool_MissingFile(t *testing.T) {
	tool := SQLFromStructTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestSQLFromStructTool_MissingStructName(t *testing.T) {
	tool := SQLFromStructTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{"file": "x.go"}, nil)
	if err == nil {
		t.Error("expected error for missing struct_name")
	}
}

func TestSQLFromStructTool_BasicStruct(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goCode := `package test

type User struct {
	ID    int    ` + "`" + `json:"id"` + "`" + `
	Name  string ` + "`" + `json:"name"` + "`" + `
	Email string
}`

	goPath := filepath.Join(dir, "model.go")
	os.WriteFile(goPath, []byte(goCode), 0644)

	tool := SQLFromStructTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file":        "model.go",
		"struct_name": "User",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "CREATE TABLE") {
		t.Error("expected CREATE TABLE in output")
	}
	if !strings.Contains(text, "id") {
		t.Error("expected 'id' column")
	}
	if !strings.Contains(text, "name") {
		t.Error("expected 'name' column")
	}
	if !strings.Contains(text, "email") {
		t.Error("expected 'email' column")
	}
	// json tag "id" should override "ID" → column name "id"
	if !strings.Contains(text, "INTEGER") {
		t.Error("expected INTEGER type for int field")
	}
}

func TestSQLFromStructTool_TypeMapping(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goCode := `package test

type Types struct {
	Flag    bool
	Count   int64
	Ratio   float64
	Label   string
}`

	goPath := filepath.Join(dir, "types.go")
	os.WriteFile(goPath, []byte(goCode), 0644)

	tool := SQLFromStructTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file":        "types.go",
		"struct_name": "Types",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	if !strings.Contains(text, "BOOLEAN") {
		t.Error("expected BOOLEAN for bool")
	}
	if !strings.Contains(text, "INTEGER") {
		t.Error("expected INTEGER for int64")
	}
	if !strings.Contains(text, "REAL") {
		t.Error("expected REAL for float64")
	}
	if !strings.Contains(text, "TEXT") {
		t.Error("expected TEXT for string")
	}
}

func TestSQLFromStructTool_StructNotFound(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goCode := `package test

type Known struct {
	X int
}`

	goPath := filepath.Join(dir, "file.go")
	os.WriteFile(goPath, []byte(goCode), 0644)

	tool := SQLFromStructTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"file":        "file.go",
		"struct_name": "Unknown",
	}, nil)
	if err == nil {
		t.Error("expected error for missing struct")
	}
}

func TestSQLFromStructTool_SnakeCaseConversion(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	goCode := `package test

type MyTable struct {
	ItemID       int
	CustomerName string
}`

	goPath := filepath.Join(dir, "snake.go")
	os.WriteFile(goPath, []byte(goCode), 0644)

	tool := SQLFromStructTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"file":        "snake.go",
		"struct_name": "MyTable",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	text := result.Content[0].Text
	// Table name should be my_table
	if !strings.Contains(text, "my_table") {
		t.Errorf("expected snake_case table name 'my_table' in: %s", text)
	}
	// Column names should be snake_case
	if !strings.Contains(text, "item_id") {
		t.Errorf("expected 'item_id' column in: %s", text)
	}
	if !strings.Contains(text, "customer_name") {
		t.Errorf("expected 'customer_name' column in: %s", text)
	}
}
