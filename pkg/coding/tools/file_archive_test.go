package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFileArchiveTool_MissingArgs(t *testing.T) {
	tool := FileArchiveTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing args")
	}
}

func TestFileArchiveTool_CreateZip(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	// Create a file to archive
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello"), 0644)

	dst := filepath.Join(dir, "out.zip")
	tool := FileArchiveTool()
	result, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "create", "source": srcDir, "dest": dst,
	}, nil)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	if result.Details["success"] != true {
		t.Errorf("expected success")
	}

	// Verify file exists
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Error("zip file not created")
	}
}

func TestFileArchiveTool_ExtractZip(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	// Create a zip first
	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("content"), 0644)

	dstZip := filepath.Join(dir, "test.zip")
	tool := FileArchiveTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{
		"action": "create", "source": srcDir, "dest": dstZip,
	}, nil)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}

	// Now extract
	extDir := filepath.Join(dir, "extracted")
	_, err = tool.Execute(context.Background(), "id", map[string]any{
		"action": "extract", "source": dstZip, "dest": extDir, "format": "zip",
	}, nil)
	if err != nil {
		t.Fatalf("extract zip: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(extDir, "test.txt"))
	if string(data) != "content" {
		t.Errorf("expected 'content', got %q", string(data))
	}
}
