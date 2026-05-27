package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestGenerateMocks_Basic(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type UserRepo interface {
	GetUser(id int) (string, error)
	SaveUser(name string, age int) error
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateMocksTool()
	params := map[string]any{
		"file":           "main.go",
		"interface_name": "UserRepo",
	}
	res, err := tool.Execute(context.Background(), "gm1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "MockUserRepo") {
		t.Errorf("expected MockUserRepo, got:\n%s", text)
	}
	if !strings.Contains(text, "GetUserFunc") {
		t.Errorf("expected GetUserFunc field, got:\n%s", text)
	}
	if !strings.Contains(text, "SaveUserFunc") {
		t.Errorf("expected SaveUserFunc field, got:\n%s", text)
	}
	if !strings.Contains(text, "func (m *MockUserRepo) GetUser") {
		t.Errorf("expected GetUser method, got:\n%s", text)
	}
	if !strings.Contains(text, "m.GetUserFunc(") {
		t.Errorf("expected call to GetUserFunc field, got:\n%s", text)
	}
}

func TestGenerateMocks_SingleMethod(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Logger interface {
	Log(msg string)
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateMocksTool()
	params := map[string]any{
		"file":           "main.go",
		"interface_name": "Logger",
	}
	res, err := tool.Execute(context.Background(), "gm2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "MockLogger") {
		t.Errorf("expected MockLogger, got:\n%s", text)
	}
	if !strings.Contains(text, "LogFunc") {
		t.Errorf("expected LogFunc field, got:\n%s", text)
	}
	if !strings.Contains(text, "func (m *MockLogger) Log") {
		t.Errorf("expected Log method, got:\n%s", text)
	}
}

func TestGenerateMocks_InterfaceNotFound(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Real struct{}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateMocksTool()
	params := map[string]any{
		"file":           "main.go",
		"interface_name": "MissingInterface",
	}
	_, err := tool.Execute(context.Background(), "gm3", params, nil)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' error, got: %v", err)
	}
}

func TestGenerateMocks_MissingParams(t *testing.T) {
	tool := tools.GenerateMocksTool()

	_, err := tool.Execute(context.Background(), "gm4", map[string]any{"file": "", "interface_name": "I"}, nil)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	_, err = tool.Execute(context.Background(), "gm5", map[string]any{"file": "x.go", "interface_name": ""}, nil)
	if err == nil {
		t.Fatal("expected error for missing interface_name")
	}
}

func TestGenerateMocks_NoMethods(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Empty interface{}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateMocksTool()
	params := map[string]any{
		"file":           "main.go",
		"interface_name": "Empty",
	}
	_, err := tool.Execute(context.Background(), "gm6", params, nil)
	if err == nil || !strings.Contains(err.Error(), "no methods") {
		t.Errorf("expected 'no methods' error, got: %v", err)
	}
}

func TestGenerateMocks_CustomPackageName(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Greeter interface {
	Greet(name string) string
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.GenerateMocksTool()
	params := map[string]any{
		"file":           "main.go",
		"interface_name": "Greeter",
		"package_name":   "mocks",
	}
	res, err := tool.Execute(context.Background(), "gm7", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "MockGreeter") {
		t.Errorf("expected MockGreeter, got:\n%s", text)
	}
	if !strings.Contains(text, "GreetFunc") {
		t.Errorf("expected GreetFunc field, got:\n%s", text)
	}
}
