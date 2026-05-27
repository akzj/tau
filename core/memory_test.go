package core

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestWorkingMemoryAddAndRecall(t *testing.T) {
	wm := NewWorkingMemory(10)
	wm.Add(Observation{Source: "tool:bash", Content: "ran go build: success", Importance: 0.8})
	wm.Add(Observation{Source: "loop:turn", Content: "task completed: fix bug", Importance: 0.7})
	results := wm.Recall("go build", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 recall result, got %d", len(results))
	}
}

func TestWorkingMemoryEviction(t *testing.T) {
	wm := NewWorkingMemory(3)
	for i := 0; i < 5; i++ {
		wm.Add(Observation{Source: "test", Content: fmt.Sprintf("entry-%d", i), Importance: float64(i) / 10})
	}
	if wm.Len() > 3 {
		t.Errorf("expected ≤3 after eviction, got %d", wm.Len())
	}
}

func TestWorkingMemorySummarize(t *testing.T) {
	wm := NewWorkingMemory(100)
	wm.Add(Observation{Source: "test", Content: "hello", Importance: 0.5})
	summary := wm.Summarize()
	if !strings.Contains(summary, "hello") {
		t.Error("expected 'hello' in summary")
	}
}

func TestEpisodicMemoryRecordAndRecall(t *testing.T) {
	dir := t.TempDir()
	em := NewEpisodicMemory(dir, 100, nil)
	em.Record(Episode{Trigger: "error", Action: "fix", Outcome: "resolved", Lesson: "check nil", Tags: []string{"bug"}})
	results := em.Recall("error", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 recall, got %d", len(results))
	}
}

func TestEpisodicMemoryLimit(t *testing.T) {
	dir := t.TempDir()
	em := NewEpisodicMemory(dir, 5, nil)
	for i := 0; i < 10; i++ {
		em.Record(Episode{Trigger: fmt.Sprintf("task-%d", i), Action: "done", Outcome: "ok", Tags: []string{}})
	}
	if em.Len() > 5 {
		t.Errorf("expected ≤5, got %d", em.Len())
	}
}

func TestEpisodicMemoryForget(t *testing.T) {
	dir := t.TempDir()
	em := NewEpisodicMemory(dir, 100, nil)
	em.Record(Episode{Trigger: "old-task", Action: "done", Outcome: "ok", Tags: []string{}})
	// Override timestamp to be old
	em.mu.Lock()
	em.episodes[0].Timestamp = time.Now().Add(-48 * time.Hour)
	em.mu.Unlock()
	count := em.Forget(24 * time.Hour)
	if count != 1 {
		t.Errorf("expected 1 forgotten, got %d", count)
	}
}

func TestEpisodicMemoryByTag(t *testing.T) {
	dir := t.TempDir()
	em := NewEpisodicMemory(dir, 100, nil)
	em.Record(Episode{Trigger: "t1", Action: "a1", Outcome: "o1", Tags: []string{"go", "bug"}})
	em.Record(Episode{Trigger: "t2", Action: "a2", Outcome: "o2", Tags: []string{"python", "feature"}})
	results := em.ListByTag("go")
	if len(results) != 1 {
		t.Errorf("expected 1 by tag 'go', got %d", len(results))
	}
}

func TestMemorySystemIntegration(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemorySystem("", dir, nil)
	ms.AddObservation("loop:turn", "completed task", 0.8)
	ms.RecordEpisode("task-start", "run build", "success", "remember to check deps", []string{"build"})

	results := ms.RecallEpisodes("build", 5)
	if len(results) != 1 {
		t.Errorf("expected 1 episode, got %d", len(results))
	}
}
