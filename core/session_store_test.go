package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionStoreSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)

	sess := &Session{
		ID:        "test-1",
		CreatedAt: time.Now(),
		Transcript: NewTranscript(),
		CWD:       "/tmp/test",
		TotalUsage: NewTokenUsage("gpt-4o", 100, 50),
		CallCount: 3,
	}
	sess.Transcript.Append(Message{Role: RoleUser, Content: "hello"})
	sess.Transcript.Append(Message{Role: RoleAssistant, Content: "hi"})

	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}

	record, err := store.Load("test-1")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if record.ID != "test-1" {
		t.Errorf("expected test-1, got %s", record.ID)
	}
	if len(record.Messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(record.Messages))
	}
	if record.CWD != "/tmp/test" {
		t.Errorf("expected /tmp/test, got %s", record.CWD)
	}
	if record.CallCount != 3 {
		t.Errorf("expected 3, got %d", record.CallCount)
	}
}

func TestSessionStoreList(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	sess := &Session{ID: "a", CreatedAt: time.Now(), Transcript: NewTranscript()}
	sess2 := &Session{ID: "b", CreatedAt: time.Now(), Transcript: NewTranscript()}
	store.Save(sess)
	store.Save(sess2)

	records, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2, got %d", len(records))
	}
}

func TestSessionStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	sess := &Session{ID: "del", CreatedAt: time.Now(), Transcript: NewTranscript()}
	store.Save(sess)

	if err := store.Delete("del"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Load("del"); err == nil {
		t.Error("expected error after delete")
	}
}

func TestSessionStoreNotFound(t *testing.T) {
	store := NewSessionStore(t.TempDir())
	_, err := store.Load("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestSessionStoreAutoCreateDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "auto-created")
	store := NewSessionStore(dir)
	sess := &Session{ID: "auto", CreatedAt: time.Now(), Transcript: NewTranscript()}
	if err := store.Save(sess); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("directory not auto-created")
	}
}

func TestSessionStoreMetaFields(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	sess := &Session{
		ID:        "meta",
		CreatedAt: time.Now(),
		Transcript: NewTranscript(),
		Summary:   "test summary",
		TotalUsage: NewTokenUsage("gpt-4o", 500, 200),
		CallCount: 5,
	}
	store.Save(sess)
	record, _ := store.Load("meta")
	if record.Summary != "test summary" {
		t.Error("summary mismatch")
	}
	if record.TotalUsage.PromptTokens != 500 {
		t.Error("usage mismatch")
	}
}

func TestSessionStorePrune(t *testing.T) {
	dir := t.TempDir()
	store := NewSessionStore(dir)
	sess := &Session{ID: "old", CreatedAt: time.Now().Add(-48 * time.Hour), Transcript: NewTranscript()}
	store.Save(sess)
	// Override updatedAt to be old
	path := filepath.Join(dir, "old.json")
	data, _ := os.ReadFile(path)
	var r SessionRecord
	json.Unmarshal(data, &r)
	r.UpdatedAt = time.Now().Add(-48 * time.Hour)
	data, _ = json.MarshalIndent(r, "", "  ")
	os.WriteFile(path, data, 0644)

	count, err := store.Prune(24 * time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("expected 1 pruned, got %d", count)
	}
}
