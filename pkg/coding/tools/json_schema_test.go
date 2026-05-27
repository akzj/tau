package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestJSONSchemaTool_MissingFile(t *testing.T) {
	tool := JSONSchemaTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing json_file")
	}
}

func TestJSONSchemaTool_SimpleObject(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	// Create a simple JSON file
	jsonContent := `{"name": "Alice", "age": 30, "active": true}`
	jsonPath := filepath.Join(dir, "test.json")
	os.WriteFile(jsonPath, []byte(jsonContent), 0644)

	tool := JSONSchemaTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"json_file": "test.json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.Content[0].Type != "text" {
		t.Error("expected text content")
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if schema["type"] != "object" {
		t.Errorf("expected type=object, got %v", schema["type"])
	}
	if _, ok := schema["properties"]; !ok {
		t.Error("expected properties in schema")
	}
	if _, ok := schema["required"]; !ok {
		t.Error("expected required in schema")
	}
}

func TestJSONSchemaTool_ArrayWithMixedTypes(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	jsonContent := `[1, "two", 3.0, null, true]`
	jsonPath := filepath.Join(dir, "mixed.json")
	os.WriteFile(jsonPath, []byte(jsonContent), 0644)

	tool := JSONSchemaTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"json_file": "mixed.json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	if schema["type"] != "array" {
		t.Errorf("expected type=array, got %v", schema["type"])
	}
}

func TestJSONSchemaTool_NestedObject(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	jsonContent := `{"user": {"name": "Bob", "address": {"city": "NYC", "zip": "10001"}}}`
	jsonPath := filepath.Join(dir, "nested.json")
	os.WriteFile(jsonPath, []byte(jsonContent), 0644)

	tool := JSONSchemaTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"json_file": "nested.json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var schema map[string]any
	if err := json.Unmarshal([]byte(result.Content[0].Text), &schema); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}

	// Should have $schema field
	if _, ok := schema["$schema"]; !ok {
		t.Error("expected $schema field")
	}
}

func TestJSONSchemaTool_WriteToFile(t *testing.T) {
	dir := t.TempDir()
	origRoot := WorkspaceRoot
	defer func() { WorkspaceRoot = origRoot }()
	WorkspaceRoot = dir

	jsonContent := `{"x": 1}`
	jsonPath := filepath.Join(dir, "data.json")
	os.WriteFile(jsonPath, []byte(jsonContent), 0644)

	tool := JSONSchemaTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"json_file":          "data.json",
		"output_schema_path": "out.json",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Check that output file exists
	outPath := filepath.Join(dir, "out.json")
	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Error("output schema file was not created")
	}

	// Verify content
	data, _ := os.ReadFile(outPath)
	var schema map[string]any
	json.Unmarshal(data, &schema)
	if schema["type"] != "object" {
		t.Errorf("expected type=object in output file, got %v", schema["type"])
	}

	_ = result
}
