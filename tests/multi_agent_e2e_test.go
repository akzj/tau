//go:build e2e

package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/akzj/tau/core"
)

// TestMultiAgentE2EStress spawns 3 sub-agents in parallel and verifies all complete.
func TestMultiAgentE2EStress(t *testing.T) {
	if os.Getenv("TAU_BIN") == "" {
		t.Skip("TAU_BIN not set — required for sub-agent spawning")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	baseDir := t.TempDir()

	specs := []core.SubAgentSpec{
		{
			ID:        "todo-scanner",
			Prompt:    "Search for TODO comments in the project and categorize them by priority.",
			Tools:     []string{"search_code", "read", "list_files", "workspace_diag"},
			Budget:    3,
			Workspace: filepath.Join(baseDir, "tau-e2e-sub-todo"),
		},
		{
			ID:        "coverage-analyzer",
			Prompt:    "Analyze test coverage gaps in the project. Report which packages need more tests.",
			Tools:     []string{"run_tests", "search_code", "read"},
			Budget:    3,
			Workspace: filepath.Join(baseDir, "tau-e2e-sub-coverage"),
		},
		{
			ID:        "structure-summarizer",
			Prompt:    "Provide a summary of the project structure: file tree, languages used, key directories.",
			Tools:     []string{"workspace_diag", "list_files"},
			Budget:    2,
			Workspace: filepath.Join(baseDir, "tau-e2e-sub-structure"),
		},
	}

	for _, spec := range specs {
		os.MkdirAll(spec.Workspace, 0755)
	}

	pool := core.NewSubAgentPool()
	for _, spec := range specs {
		if err := pool.Spawn(spec); err != nil {
			t.Fatalf("spawn %s: %v", spec.ID, err)
		}
	}

	t.Logf("Launched %d sub-agents, awaiting completion...", len(specs))
	start := time.Now()

	results, err := pool.Collect(ctx)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	elapsed := time.Since(start)
	t.Logf("All sub-agents completed in %v", elapsed)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("sub-agent %s failed: %v", r.ID, r.Err)
		}
		if r.Output == "" {
			t.Errorf("sub-agent %s returned empty output", r.ID)
		}
		t.Logf("[%s] output (%d bytes, %v): %.200s...", r.ID, len(r.Output), r.Duration, r.Output)
	}

	for _, r := range results {
		if r.Duration > 120*time.Second {
			t.Errorf("sub-agent %s took too long: %v", r.ID, r.Duration)
		}
	}

	var totalDuration time.Duration
	for _, r := range results {
		totalDuration += r.Duration
	}
	if elapsed >= totalDuration {
		t.Logf("note: parallel elapsed (%v) >= sum of durations (%v) — may indicate sequential execution",
			elapsed, totalDuration)
	} else {
		t.Logf("parallel speedup: %.1fx (sum=%v, elapsed=%v)",
			float64(totalDuration)/float64(elapsed), totalDuration, elapsed)
	}
}

// ensure fmt import used
var _ = fmt.Sprintf
