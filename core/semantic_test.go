package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSemanticAddAndQuery(t *testing.T) {
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "avoid nil dereference in file readers", Tags: []string{"go", "bug", "nil"}, Confidence: 0.8})
	sm.Add(SemanticFact{Content: "use defer for cleanup", Tags: []string{"go", "pattern"}, Confidence: 0.9})

	results := sm.Query("nil", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if !strings.Contains(results[0].Content, "nil") {
		t.Error("expected nil in result")
	}
}

func TestSemanticList(t *testing.T) {
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "fact1", Tags: []string{"a"}})
	sm.Add(SemanticFact{Content: "fact2", Tags: []string{"b"}})
	if sm.Len() != 2 {
		t.Errorf("expected 2 facts, got %d", sm.Len())
	}
}

func TestSemanticPrune(t *testing.T) {
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "high quality", Confidence: 0.9, Tags: []string{}})
	sm.Add(SemanticFact{Content: "low quality", Confidence: 0.2, Tags: []string{}})
	removed := sm.Prune(0.5)
	if removed != 1 {
		t.Errorf("expected 1 pruned, got %d", removed)
	}
	if sm.Len() != 1 {
		t.Errorf("expected 1 remaining, got %d", sm.Len())
	}
}

func TestSemanticPersistReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sem.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sm := NewSemanticMemory(store)
	sm.Add(SemanticFact{Content: "persisted fact", Tags: []string{"test"}, Confidence: 0.8})

	// Reload
	sm2 := NewSemanticMemory(store)
	results := sm2.Query("persisted", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 persisted fact, got %d", len(results))
	}
}

func TestSimpleExtractor(t *testing.T) {
	ext := &SimpleExtractor{}
	facts, err := ext.Extract(context.Background(), Episode{
		ID: "ep-1", Trigger: "error: nil pointer in read.go", Action: "added nil check",
		Outcome: "resolved", Lesson: "always check nil before deref", Tags: []string{"bug"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) == 0 {
		t.Error("expected at least 1 extracted fact")
	}
	found := false
	for _, f := range facts {
		if strings.Contains(f.Content, "nil") {
			found = true
		}
	}
	if !found {
		t.Error("expected nil-related fact")
	}
}

func TestSimpleExtractorEmpty(t *testing.T) {
	ext := &SimpleExtractor{}
	facts, _ := ext.Extract(context.Background(), Episode{
		ID: "ep-2", Trigger: "task", Action: "done", Outcome: "ok", Tags: []string{},
	})
	if len(facts) == 0 {
		t.Error("expected at least 1 generic fact")
	}
}

func TestMemorySystemExtractionChain(t *testing.T) {
	ms := NewMemorySystem("", t.TempDir(), nil)
	ms.RecordEpisode("error: timeout", "added retry", "resolved", "add retry logic to network calls", []string{"network", "bug"})

	// Allow async extraction to complete
	time.Sleep(100 * time.Millisecond)

	if ms.SemMem.Len() < 1 {
		t.Errorf("expected ≥1 extracted facts, got %d (extraction may still be running)", ms.SemMem.Len())
	}

	facts := ms.SemanticQuery("timeout", 5)
	if len(facts) < 1 {
		t.Error("expected timeout-related facts")
	}
}

func TestSemanticBackwardCompat(t *testing.T) {
	// SemanticMemory with nil store should work (in-memory only)
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "test", Tags: []string{}})
	if sm.Len() != 1 {
		t.Error("expected 1 fact")
	}
}

func TestSemanticConcurrent(t *testing.T) {
	sm := NewSemanticMemory(nil)
	done := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		go func(n int) {
			sm.Add(SemanticFact{Content: fmt.Sprintf("fact-%d", n), Tags: []string{}})
			done <- true
		}(i)
	}
	for i := 0; i < 20; i++ {
		<-done
	}
	if sm.Len() != 20 {
		t.Errorf("expected 20, got %d", sm.Len())
	}
}

func TestSemanticTagQuery(t *testing.T) {
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "use goroutines for parallelism", Tags: []string{"go", "concurrency"}})
	sm.Add(SemanticFact{Content: "prefer list comprehensions", Tags: []string{"python", "pattern"}})

	results := sm.Query("concurrency", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 by tag, got %d", len(results))
	}
}

func TestSemanticNoExtractOnEmpty(t *testing.T) {
	ms := NewMemorySystem("", t.TempDir(), nil)
	ms.RecordEpisode("", "fix", "done", "", []string{})
	time.Sleep(50 * time.Millisecond)
	if ms.SemMem.Len() < 1 {
		t.Error("expected at least 1 generic fact from empty episode")
	}
}

func TestSemanticReplaceStore(t *testing.T) {
	sm := NewSemanticMemory(nil)
	sm.Add(SemanticFact{Content: "in-mem fact", Tags: []string{"mem"}})

	dir := t.TempDir()
	path := filepath.Join(dir, "sem2.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Replace store — should reload facts from store (empty) and persist new ones
	sm.ReplaceStore(store)
	sm.Add(SemanticFact{Content: "stored fact", Tags: []string{"db"}})

	// Verify via a new SemanticMemory reading same store
	sm2 := NewSemanticMemory(store)
	if sm2.Len() < 1 {
		t.Error("expected stored fact to persist")
	}
}

func TestMemorySystemSemanticQueryNilSafety(t *testing.T) {
	ms := NewMemorySystem("", t.TempDir(), nil)
	ms.SemMem = nil
	facts := ms.SemanticQuery("anything", 5)
	if facts != nil {
		t.Error("expected nil result when SemMem is nil")
	}
}
