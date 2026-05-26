package persist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akzj/tau/core"
)

func TestFork(t *testing.T) {
	entries := []core.TreeEntry{
		{ID: "e1", ParentID: "", Type: core.EntryMessage, Timestamp: time.Now(), Data: "msg1"},
		{ID: "e2", ParentID: "e1", Type: core.EntryMessage, Timestamp: time.Now(), Data: "msg2"},
	}
	id, err := Fork("source-1", entries)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filepath.Join(mustDirFork(t), id+".tree.jsonl"))

	loaded, err := LoadTree(id)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Errorf("expected 2 entries, got %d", len(loaded))
	}
	if loaded[0].ID != "e1" {
		t.Errorf("expected e1, got %s", loaded[0].ID)
	}
}

func mustDirFork(t *testing.T) string {
	t.Helper()
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	return dir
}
