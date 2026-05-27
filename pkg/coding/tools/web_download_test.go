package tools

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWebDownloadTool_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("hello download"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	WorkspaceRoot = dir
	dest := filepath.Join(dir, "out.txt")

	tool := WebDownloadTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": srv.URL, "dest": dest,
	}, nil)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}

	data, _ := os.ReadFile(dest)
	if string(data) != "hello download" {
		t.Errorf("expected 'hello download', got %q", string(data))
	}
}

func TestWebDownloadTool_MissingURL(t *testing.T) {
	tool := WebDownloadTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"dest": "/tmp/x",
	}, nil)
	if err == nil {
		t.Error("expected error for missing URL")
	}
}

func TestWebDownloadTool_BadURL(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	tool := WebDownloadTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": "http://127.0.0.1:1/nonexistent", "dest": filepath.Join(dir, "out.txt"),
	}, nil)
	if err == nil {
		t.Error("expected error for bad URL")
	}
}

func TestWebDownloadTool_Resume(t *testing.T) {
	content := strings.Repeat("data", 256)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			w.WriteHeader(http.StatusPartialContent)
		}
		w.Write([]byte(content))
	}))
	defer srv.Close()

	dir := t.TempDir()
	WorkspaceRoot = dir
	dest := filepath.Join(dir, "resume.txt")

	// First write partial content
	os.WriteFile(dest, []byte("partial:"), 0644)

	tool := WebDownloadTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"url": srv.URL, "dest": dest, "resume": true,
	}, nil)
	if err != nil {
		t.Fatalf("resume download failed: %v", err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}
}
