package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileChecksumTool_SHA256(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	tool := FileChecksumTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path, "algorithm": "sha256",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}
	// SHA256 of "hello" = 2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
	expected := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if result.Details["checksum"] != expected {
		t.Errorf("expected %s, got %v", expected, result.Details["checksum"])
	}
}

func TestFileChecksumTool_MD5(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir
	path := filepath.Join(dir, "test.txt")
	os.WriteFile(path, []byte("hello"), 0644)

	tool := FileChecksumTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": path, "algorithm": "md5",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// MD5 of "hello" = 5d41402abc4b2a76b9719d911017c592
	expected := "5d41402abc4b2a76b9719d911017c592"
	if result.Details["checksum"] != expected {
		t.Errorf("expected %s, got %v", expected, result.Details["checksum"])
	}
}

func TestFileChecksumTool_MissingPath(t *testing.T) {
	tool := FileChecksumTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestFileChecksumTool_FileNotFound(t *testing.T) {
	tool := FileChecksumTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"path": "/nonexistent/file.txt",
	}, nil)
	if err == nil {
		t.Error("expected error for missing file")
	}
}
