package core

import (
	"strings"
	"testing"
)

func TestMemoryIntegration_WorkingMemoryInLoop(t *testing.T) {
	dir := t.TempDir()
	sess := &Session{
		ID:        "test-mem",
		Transcript: NewTranscript(),
		Memory:    NewMemorySystem(dir, dir),
		Tools:     NewToolRegistry(),
		Providers: NewProviderRegistry(),
	}
	sess.Memory.AddObservation("user", "open main.go", 0.9)
	sess.Memory.AddObservation("tool:read", "read main.go: 42 lines", 0.5)
	sess.Memory.AddObservation("tool:edit", "edited main.go: added comment", 0.5)

	summary := sess.Memory.Working.Summarize()
	if !strings.Contains(summary, "open main.go") {
		t.Error("expected user observation in summary")
	}
	if !strings.Contains(summary, "read main.go") {
		t.Error("expected tool observation in summary")
	}

	sess.Memory.AddObservation("user", "run go build", 0.9)
	sess.Memory.AddObservation("tool:bash", "build: success", 0.5)

	summary2 := sess.Memory.Working.Summarize()
	if !strings.Contains(summary2, "run go build") {
		t.Error("expected turn-3 observation in summary")
	}
}

func TestMemoryIntegration_EpisodicLearning(t *testing.T) {
	dir := t.TempDir()
	ms := NewMemorySystem(dir, dir)

	ms.RecordEpisode(
		"error: nil pointer dereference in read.go:42",
		"added nil check before dereference",
		"resolved — no more nil pointer panics",
		"always check pointer before dereference in file readers",
		[]string{"error", "nil", "read"},
	)

	episodes := ms.RecallEpisodes("nil pointer", 5)
	if len(episodes) == 0 {
		t.Error("expected recalled episodes for nil pointer error")
	}
	if !strings.Contains(episodes[0].Lesson, "check pointer") {
		t.Error("expected nil-check lesson")
	}

	ms.RecordEpisode(
		"error: nil pointer at write.go:103",
		"added nil check before write",
		"resolved",
		"same nil-pointer pattern as read.go — add nil-check helper",
		[]string{"error", "nil", "write"},
	)

	episodes = ms.RecallEpisodes("nil pointer", 5)
	if len(episodes) < 2 {
		t.Errorf("expected >=2 episodes, got %d", len(episodes))
	}
}
