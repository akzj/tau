package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileWatchTool_MissingPath(t *testing.T) {
	tool := FileWatchTool()
	_, err := tool.Execute(context.Background(), "id", map[string]any{}, nil)
	if err == nil {
		t.Error("expected error for missing path")
	}
}

func TestFileWatchTool_WatchDir(t *testing.T) {
	dir := t.TempDir()
	WorkspaceRoot = dir

	tool := FileWatchTool()
	// Run watch in background for a short time
	done := make(chan struct{})
	go func() {
		_, err := tool.Execute(context.Background(), "id", map[string]any{
			"path": dir, "timeout": float64(2),
		}, nil)
		if err != nil {
			t.Errorf("watch failed: %v", err)
		}
		close(done)
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Create a file to trigger event
	os.WriteFile(filepath.Join(dir, "newfile.txt"), []byte("hello"), 0644)

	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Error("watch timed out")
	}
}
