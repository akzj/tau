package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestApiSurfaceTool_ExportedSymbols(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "fmt"

// Greeter is an interface that greets.
type Greeter interface {
	Greet() string
}

// Server handles requests.
type Server struct {
	Addr string
	port int
}

// NewServer creates a Server.
func NewServer(addr string) *Server {
	return &Server{Addr: addr}
}

// unexported helper
func validate(addr string) bool {
	return addr != ""
}

// Start begins listening.
func (s *Server) Start() error {
	fmt.Println("started")
	return nil
}

func (s *Server) stop() error {
	return nil
}

// DefaultPort is the default listen port.
const DefaultPort = 8080

// ErrTimeout is returned on timeout.
var ErrTimeout = fmt.Errorf("timeout")
`)

	tool := tools.ApiSurfaceTool()
	params := map[string]any{
		"include_internal": false,
	}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	// Should list exported symbols
	if !strings.Contains(text, "Server") {
		t.Errorf("expected 'Server' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "NewServer") {
		t.Errorf("expected 'NewServer' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "DefaultPort") {
		t.Errorf("expected 'DefaultPort' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "ErrTimeout") {
		t.Errorf("expected 'ErrTimeout' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "Greeter") {
		t.Errorf("expected 'Greeter' in output, got:\n%s", text)
	}
	if !strings.Contains(text, "Start") {
		t.Errorf("expected 'Start' method in output, got:\n%s", text)
	}
	// Unexported should NOT appear
	if strings.Contains(text, "validate") {
		t.Errorf("expected 'validate' to NOT appear (unexported), got:\n%s", text)
	}
	if strings.Contains(text, "stop") {
		t.Errorf("expected 'stop' to NOT appear (unexported), got:\n%s", text)
	}
}

func TestApiSurfaceTool_IncludeInternal(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

// ExportedFunc does things.
func ExportedFunc() {}

func unexportedFunc() {}
`)

	tool := tools.ApiSurfaceTool()
	params := map[string]any{
		"include_internal": true,
	}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "ExportedFunc") {
		t.Errorf("expected 'ExportedFunc', got:\n%s", text)
	}
	if !strings.Contains(text, "unexportedFunc") {
		t.Errorf("expected 'unexportedFunc' with include_internal, got:\n%s", text)
	}
}

func TestApiSurfaceTool_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	// Empty directory with no .go files
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ApiSurfaceTool()
	res, err := tool.Execute(context.Background(), "call3", nil, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No Go source files found") {
		t.Errorf("expected 'No Go source files found', got: %s", res.Content[0].Text)
	}
}

func TestApiSurfaceTool_FiltersTestFiles(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "lib.go"), `package test

func Production() {}
`)
	writeFile(t, filepath.Join(dir, "lib_test.go"), `package test

func TestHelper() {}
`)

	tool := tools.ApiSurfaceTool()
	params := map[string]any{
		"include_internal": false,
	}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "Production") {
		t.Errorf("expected 'Production', got:\n%s", text)
	}
	// TestHelper is exported, but in _test.go; should be excluded by default
	if strings.Contains(text, "TestHelper") {
		t.Errorf("expected 'TestHelper' to be excluded (in _test.go), got:\n%s", text)
	}
}

func TestApiSurfaceTool_WithPath(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	writeFile(t, filepath.Join(dir, "sub", "pkg.go"), `package sub

func SubFunc() {}
`)

	tool := tools.ApiSurfaceTool()
	params := map[string]any{
		"path": "sub",
	}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "SubFunc") {
		t.Errorf("expected 'SubFunc' in subdirectory, got:\n%s", text)
	}
}
