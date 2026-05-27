package tools_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestListSymbols_AllKinds(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

import "fmt"

const Greeting = "hello"
var Count int

type Person struct {
    Name string
}

type Greeter interface {
    Greet() string
}

func Hello() string {
    return Greeting
}

func (p *Person) SayHi() string {
    return "hi"
}

func main() {
    fmt.Println(Hello())
}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ListSymbolsTool()
	res, err := tool.Execute(context.Background(), "call1", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	// All these should appear
	for _, want := range []string{
		"Greeting (const)",
		"Count (var)",
		"Person (type)",
		"Greeter (interface)",
		"Hello (func)",
		"(*Person).SayHi (method)",
		"main (func)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, text)
		}
	}

	// Exported check
	if !strings.Contains(text, "[exported]") {
		t.Error("expected some exported symbols")
	}
}

func TestListSymbols_TypeFilter(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "app.go"), `
package app

const X = 1
var Y = 2
type T struct{}
func F() {}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ListSymbolsTool()
	params := map[string]any{"type_filter": "func,type"}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if strings.Contains(text, "(const)") {
		t.Error("const should be filtered out")
	}
	if strings.Contains(text, "(var)") {
		t.Error("var should be filtered out")
	}
	if !strings.Contains(text, "F (func)") {
		t.Error("func should be present")
	}
	if !strings.Contains(text, "T (type)") {
		t.Error("type should be present")
	}
}

func TestListSymbols_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "go.mod"), "module test")
	// No .go files

	tool := tools.ListSymbolsTool()
	res, err := tool.Execute(context.Background(), "call3", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.Contains(res.Content[0].Text, "No symbols found") {
		t.Errorf("expected 'No symbols found', got: %s", res.Content[0].Text)
	}
}

func TestListSymbols_SkipsTestFiles(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir

	writeFile(t, filepath.Join(dir, "main.go"), `
package main

func RealFunc() {}
`)
	writeFile(t, filepath.Join(dir, "main_test.go"), `
package main

func TestFake() {}
`)
	writeFile(t, filepath.Join(dir, "go.mod"), "module test")

	tool := tools.ListSymbolsTool()
	res, err := tool.Execute(context.Background(), "call4", map[string]any{}, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text

	if !strings.Contains(text, "RealFunc") {
		t.Error("RealFunc should be present (non-test file)")
	}
	if strings.Contains(text, "TestFake") {
		t.Error("TestFake should be skipped (test file)")
	}
}

func init() {
	_ = os.MkdirAll
}
