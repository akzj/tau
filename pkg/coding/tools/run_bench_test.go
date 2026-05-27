package tools_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akzj/tau/pkg/coding/tools"
)

func TestRunBenchTool_Basic(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "bench_test.go"), "package test\n\nimport \"testing\"\n\nfunc BenchmarkHello(b *testing.B) { for i := 0; i < b.N; i++ { _ = 1+1 } }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunBenchTool()
	params := map[string]any{"benchmark": ".", "timeout": "60s"}
	res, err := tool.Execute(context.Background(), "call1", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "BenchmarkHello") {
		t.Fatalf("expected 'BenchmarkHello' in result, got: %s", text)
	}
}

func TestRunBenchTool_Defaults(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "bench_test.go"), "package test\n\nimport \"testing\"\n\nfunc BenchmarkFast(b *testing.B) { for i := 0; i < b.N; i++ { _ = 1+1 } }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunBenchTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call2", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "BenchmarkFast") {
		t.Fatalf("expected 'BenchmarkFast' in result, got: %s", text)
	}
}

func TestRunBenchTool_BadTimeoutDefaults(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "bench_test.go"), "package test\n\nimport \"testing\"\n\nfunc BenchmarkOk(b *testing.B) { for i := 0; i < b.N; i++ { _ = 1+1 } }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunBenchTool()
	params := map[string]any{"timeout": "invalid"}
	res, err := tool.Execute(context.Background(), "call3", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "BenchmarkOk") {
		t.Fatalf("expected 'BenchmarkOk' in result, got: %s", text)
	}
}

func TestRunBenchTool_NoBenchmarks(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "plain_test.go"), "package test\n\nimport \"testing\"\n\nfunc TestOnly(t *testing.T) {}\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunBenchTool()
	params := map[string]any{}
	res, err := tool.Execute(context.Background(), "call4", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if strings.Contains(text, "Benchmark") {
		t.Fatalf("expected no benchmarks in output, got: %s", text)
	}
}

func TestRunBenchTool_SpecificBenchmark(t *testing.T) {
	dir := t.TempDir()
	tools.WorkspaceRoot = dir
	writeFile(t, filepath.Join(dir, "bench_test.go"), "package test\n\nimport \"testing\"\n\nfunc BenchmarkAlpha(b *testing.B) { for i := 0; i < b.N; i++ { _ = 1+1 } }\nfunc BenchmarkBeta(b *testing.B) { for i := 0; i < b.N; i++ { _ = 2+2 } }\n")
	writeFile(t, filepath.Join(dir, "go.mod"), "module test\n\ngo 1.21\n")

	tool := tools.RunBenchTool()
	params := map[string]any{"benchmark": "Alpha", "timeout": "60s"}
	res, err := tool.Execute(context.Background(), "call5", params, nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	text := res.Content[0].Text
	if !strings.Contains(text, "Alpha") {
		t.Fatalf("expected 'Alpha' in result, got: %s", text)
	}
	if strings.Contains(text, "Beta") {
		t.Fatalf("expected Beta NOT in result, got: %s", text)
	}
}
