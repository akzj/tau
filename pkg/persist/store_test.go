package persist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akzj/tau/core"
)

func TestSaveLoadCycle(t *testing.T) {
	msgs := []core.Message{
		{Role: core.RoleUser, Content: "hello"},
		{Role: core.RoleAssistant, Content: "hi!"},
		{Role: core.RoleUser, Content: "how are you"},
	}
	id := "test-" + time.Now().Format("150405")

	// Save
	if err := Save(id, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	defer os.Remove(filepath.Join(mustDir(t), id+".jsonl"))

	// Load
	loaded, err := Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != len(msgs) {
		t.Errorf("expected %d messages, got %d", len(msgs), len(loaded))
	}
	if loaded[0].Content != "hello" {
		t.Errorf("expected 'hello', got %q", loaded[0].Content)
	}
}

func TestCorruptLineRecovery(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	id := "test-corrupt-" + time.Now().Format("150405")
	path := filepath.Join(dir, id+".jsonl")

	// Write a file with one good line, one corrupt line, one good line
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	f.WriteString(`{"Role":"user","Content":"good1"}` + "\n")
	f.WriteString(`this is not json` + "\n")
	f.WriteString(`{"Role":"user","Content":"good2"}` + "\n")
	f.Close()
	defer os.Remove(path)

	msgs, err := Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// Should get 2 good messages, corrupt line skipped
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages (corrupt skipped), got %d", len(msgs))
	}
}

func TestAtomicWrite(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	id := "test-atomic-" + time.Now().Format("150405")
	path := filepath.Join(dir, id+".jsonl")
	tmpPath := path + ".tmp"

	msgs := []core.Message{{Role: core.RoleUser, Content: "test"}}

	if err := Save(id, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	defer os.Remove(path)

	// Verify: .jsonl exists, .tmp does not
	if _, err := os.Stat(path); err != nil {
		t.Errorf(".jsonl not found after Save: %v", err)
	}
	if _, err := os.Stat(tmpPath); err == nil {
		t.Errorf(".tmp should not exist after successful Save")
	}
}

func TestCWD(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	id := "test-cwd-" + time.Now().Format("150405")
	cwd := "/home/test"

	if err := SaveCWD(id, cwd); err != nil {
		t.Fatalf("SaveCWD: %v", err)
	}
	defer os.Remove(filepath.Join(dir, id+".cwd"))

	loaded := LoadCWD(id)
	if loaded != cwd {
		t.Errorf("expected CWD %q, got %q", cwd, loaded)
	}
}

func TestSessionInfoCWD(t *testing.T) {
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	id := "test-info-" + time.Now().Format("150405")
	cwd := "/home/info-test"

	// Save a session with CWD
	msgs := []core.Message{{Role: core.RoleUser, Content: "test"}}
	if err := Save(id, msgs); err != nil {
		t.Fatalf("Save: %v", err)
	}
	defer os.Remove(filepath.Join(dir, id+".jsonl"))

	if err := SaveCWD(id, cwd); err != nil {
		t.Fatalf("SaveCWD: %v", err)
	}
	defer os.Remove(filepath.Join(dir, id+".cwd"))

	// List should include CWD
	infos, err := List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	found := false
	for _, info := range infos {
		if info.ID == id && info.CWD == cwd {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("session %s with CWD %q not found in List()", id, cwd)
	}
}

func mustDir(t *testing.T) string {
	t.Helper()
	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir: %v", err)
	}
	return dir
}
