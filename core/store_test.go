package core

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreCRUD(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	err = store.Write("key1", []byte("value1"))
	if err != nil {
		t.Fatal(err)
	}

	val, err := store.Read("key1")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != "value1" {
		t.Errorf("got %q, want value1", val)
	}

	keys, err := store.List("key")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Errorf("expected 1 key, got %d", len(keys))
	}

	err = store.Delete("key1")
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Read("key1")
	if err == nil {
		t.Error("expected not found error")
	}
}

func TestStoreEpisodicPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ep.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	em := NewEpisodicMemory("", 100, store)
	em.Record(Episode{Trigger: "error", Action: "fix", Outcome: "ok", Lesson: "test", Tags: []string{"bug"}})

	// Verify recall works immediately
	results := em.Recall("error", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 episode, got %d", len(results))
	}

	// Create new EM with same store — episodes should persist (cross restart)
	em2 := NewEpisodicMemory("", 100, store)
	results2 := em2.Recall("error", 5)
	if len(results2) != 1 {
		t.Errorf("expected 1 persisted episode after reload, got %d", len(results2))
	}
}

func TestStoreLRUCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lru.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	em := NewEpisodicMemory("", 1000, store)
	// Add 150 episodes — LRU should keep last 100 in cache
	for i := 0; i < 150; i++ {
		em.Record(Episode{Trigger: fmt.Sprintf("t%d", i), Action: "a", Outcome: "ok", Tags: []string{}})
	}
	// Recall should still work via store even if cache evicted oldest
	results := em.Recall("t149", 1)
	if len(results) != 1 {
		t.Errorf("expected recent episode, got %d", len(results))
	}
}

func TestStoreNoPersist(t *testing.T) {
	// When store is nil, behavior is identical to current in-memory only
	em := NewEpisodicMemory("", 100, nil)
	em.Record(Episode{Trigger: "test", Action: "a", Outcome: "ok", Tags: []string{}})
	results := em.Recall("test", 5)
	if len(results) != 1 {
		t.Error("expected 1 episode in memory")
	}

	// New EM without store should have zero episodes
	em2 := NewEpisodicMemory("", 100, nil)
	if len(em2.Recall("test", 5)) != 0 {
		t.Error("expected 0 episodes without persistence")
	}
}

func TestStoreConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "concurrent.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			// Retry on busy errors — SQLite WAL supports concurrent readers
			// but writers may contend briefly.
			for attempt := 0; attempt < 5; attempt++ {
				mu.Lock()
				err := store.Write(fmt.Sprintf("k%d", n), []byte(fmt.Sprintf("v%d", n)))
				mu.Unlock()
				if err == nil {
					return
				}
			}
			errs <- fmt.Errorf("goroutine %d: all attempts failed", n)
		}(i)
	}
	wg.Wait()
	close(errs)

	for e := range errs {
		t.Error(e)
	}

	keys, err := store.List("k")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) < 10 {
		t.Errorf("expected 10 keys, got %d: %v", len(keys), keys)
	}
}

func TestStoreWorkingMemoryPersist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wm.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Save working memory state
	data := []byte(`{"observations":["hello","world"]}`)
	if err := store.Write("working_memory", data); err != nil {
		t.Fatal(err)
	}

	// Read back
	val, err := store.Read("working_memory")
	if err != nil {
		t.Fatal(err)
	}
	if string(val) != string(data) {
		t.Errorf("got %q, want %q", val, data)
	}
}

func TestStoreAutoMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "migrate.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	// Second open should not fail (tables already exist)
	store.Close()
	store2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	store2.Close()
}

func TestStoreEmptyEpisode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty_ep.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Empty-tags episode should persist and reload correctly
	em := NewEpisodicMemory("", 100, store)
	em.Record(Episode{Trigger: "trig", Action: "act", Outcome: "ok", Lesson: "learned", Tags: []string{}})

	em2 := NewEpisodicMemory("", 100, store)
	results := em2.Recall("trig", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 episode, got %d", len(results))
	}
	if len(results[0].Tags) != 0 {
		t.Errorf("expected empty tags, got %v", results[0].Tags)
	}
}

func TestStoreMultipleKeys(t *testing.T) {
	path := filepath.Join(t.TempDir(), "multikey.db")
	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	// Write keys with different prefixes
	store.Write("wm:state", []byte("s1"))
	store.Write("wm:observations", []byte("s2"))
	store.Write("ep:latest", []byte("s3"))

	keys, err := store.List("wm:")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 wm keys, got %d: %v", len(keys), keys)
	}
}
