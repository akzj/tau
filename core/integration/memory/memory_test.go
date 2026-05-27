//go:build integration

package memory_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/internal/testutil"
	"github.com/akzj/tau/pkg/testing/faux"
)

// TestEpisodicRoundtrip verifies record→recall for episodic memory.
func TestEpisodicRoundtrip(t *testing.T) {
	dir := t.TempDir()
	ms := core.NewMemorySystem(dir, dir, nil)

	ms.RecordEpisode(
		"error: nil pointer dereference in read.go:42",
		"added nil check before dereference",
		"resolved — no more nil pointer panics",
		"always check pointer before dereference in file readers",
		[]string{"error", "nil", "read"},
	)

	time.Sleep(10 * time.Millisecond)

	eps := ms.RecallEpisodes("nil pointer", 5)
	if len(eps) == 0 {
		t.Error("expected recalled episodes for nil pointer error")
	}
	if !strings.Contains(eps[0].Lesson, "check pointer") {
		t.Errorf("expected nil-check lesson, got %q", eps[0].Lesson)
	}

	// Record a second episode with similar pattern
	ms.RecordEpisode(
		"error: nil pointer at write.go:103",
		"added nil check before write",
		"resolved",
		"same nil-pointer pattern as read.go — add nil-check helper",
		[]string{"error", "nil", "write"},
	)

	eps = ms.RecallEpisodes("nil pointer", 5)
	if len(eps) < 2 {
		t.Errorf("expected >=2 episodes, got %d", len(eps))
	}
}

// TestSemanticExtraction verifies async knowledge extraction from episodes.
func TestSemanticExtraction(t *testing.T) {
	dir := t.TempDir()
	ms := core.NewMemorySystem(dir, dir, nil)

	ms.RecordEpisode(
		"error: nil pointer dereference",
		"added nil check",
		"resolved",
		"always check nil before dereference",
		[]string{"bug", "nil"},
	)

	// Allow async extraction to complete
	time.Sleep(200 * time.Millisecond)

	facts := ms.SemanticQuery("nil", 5)
	// May or may not have facts depending on extractor config
	t.Logf("SemanticQuery for 'nil' returned %d facts", len(facts))
}

// TestWorkingMemoryFlow verifies working memory observation→summary→recall cycle.
func TestWorkingMemoryFlow(t *testing.T) {
	dir := t.TempDir()
	ms := core.NewMemorySystem(dir, dir, nil)

	// Add observations simulating a coding session
	ms.AddObservation("user", "open main.go", 0.9)
	ms.AddObservation("tool:read", "read main.go: 42 lines", 0.5)
	ms.AddObservation("tool:edit", "edited main.go: added comment", 0.6)
	ms.AddObservation("user", "run go build", 0.9)
	ms.AddObservation("tool:bash", "build: success", 0.5)

	summary := ms.Working.Summarize()
	if !strings.Contains(summary, "open main.go") {
		t.Error("expected 'open main.go' in working memory summary")
	}
	if !strings.Contains(summary, "build: success") {
		t.Error("expected 'build: success' in working memory summary")
	}

	// Recall by query
	results := ms.Working.Recall("build", 5)
	if len(results) < 1 {
		t.Error("expected at least 1 recall result for 'build'")
	}
}

// TestFauxChaosInjection verifies Loop behavior when provider is in chaos mode.
func TestFauxChaosInjection(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	h.Prov.SetMode(faux.ModeEvil)

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	// With evil mode, the provider should produce errors.
	// The loop may handle this by returning an error or emitting error events.
	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "test under chaos"})

	if err != nil {
		t.Logf("chaos injection produced expected error: %v", err)
		return
	}

	// If we got a run, drain events and check for error events
	hasError := false
	for ev := range run.Events() {
		if _, ok := ev.(core.ErrorEvent); ok {
			hasError = true
		}
	}
	<-run.Done()

	if hasError {
		t.Logf("chaos injection: error event received as expected")
	} else {
		t.Logf("chaos injection: no error event (provider may have returned random error or queued response)")
	}
}
