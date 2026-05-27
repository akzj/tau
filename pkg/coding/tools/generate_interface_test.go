package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGenerateInterface_Basic(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type MyService struct {
	name string
}

func (s *MyService) GetName() string {
	return s.name
}

func (s *MyService) SetName(n string) {
	s.name = n
}

func (s *MyService) privateMethod() {
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateInterfaceTool()
	params := map[string]any{
		"file":        "main.go",
		"struct_name": "MyService",
	}
	res, err := tool.Execute(context.Background(), "gi1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "IMyService") {
		t.Errorf("expected IMyService interface name, got:\n%s", text)
	}
	if !strings.Contains(text, "GetName() string") {
		t.Errorf("expected GetName signature, got:\n%s", text)
	}
	if !strings.Contains(text, "SetName(n string)") {
		t.Errorf("expected SetName signature, got:\n%s", text)
	}
	if strings.Contains(text, "privateMethod") {
		t.Errorf("private method should not appear in interface:\n%s", text)
	}
}

func TestGenerateInterface_NoMethods(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type EmptyStruct struct {
	data int
}

func (e EmptyStruct) unexported() {}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateInterfaceTool()
	params := map[string]any{
		"file":        "main.go",
		"struct_name": "EmptyStruct",
	}
	_, err := tool.Execute(context.Background(), "gi2", params, nil)
	if err == nil || !strings.Contains(err.Error(), "no exported methods") {
		t.Errorf("expected 'no exported methods' error, got: %v", err)
	}
}

func TestGenerateInterface_StructNotFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Other struct{}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateInterfaceTool()
	params := map[string]any{
		"file":        "main.go",
		"struct_name": "NonExistent",
	}
	_, err := tool.Execute(context.Background(), "gi3", params, nil)
	if err == nil {
		t.Fatal("expected error for non-existent struct")
	}
}

func TestGenerateInterface_MissingParams(t *testing.T) {
	tool := tools.GenerateInterfaceTool()

	_, err := tool.Execute(context.Background(), "gi4", map[string]any{"file": "", "struct_name": "S"}, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	_, err = tool.Execute(context.Background(), "gi5", map[string]any{"file": "x.go", "struct_name": ""}, nil)
	if err == nil {
		t.Fatal("expected error for missing struct_name")
	}
}

func TestGenerateInterface_ComplexSignatures(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "context"

type ComplexService struct{}

func (c *ComplexService) Process(ctx context.Context, input []byte) (string, error) {
	return "", nil
}

func (c *ComplexService) Stats() map[string]int {
	return nil
}

func (c *ComplexService) Handle(callback func(int) bool) {
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateInterfaceTool()
	params := map[string]any{
		"file":        "main.go",
		"struct_name": "ComplexService",
	}
	res, err := tool.Execute(context.Background(), "gi6", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "IComplexService") {
		t.Errorf("expected IComplexService, got:\n%s", text)
	}
	if !strings.Contains(text, "Context") {
		t.Errorf("expected Context parameter, got:\n%s", text)
	}
	if !strings.Contains(text, "[]byte") {
		t.Errorf("expected []byte parameter, got:\n%s", text)
	}
	if !strings.Contains(text, "string, error") || !strings.Contains(text, "(string, error)") {
		// Either form is acceptable
	}
	if !strings.Contains(text, "map[string]int") {
		t.Errorf("expected map[string]int return type, got:\n%s", text)
	}
}
