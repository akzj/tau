//go:build !no_plugins

package core

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestSubAgentPoolSpawnCollect(t *testing.T) {
	if os.Getenv("TAU_BIN") == "" {
		t.Skip("TAU_BIN not set — set to tau binary for sub-agent tests")
	}

	pool := NewSubAgentPool()

	for i, msg := range []string{"hello", "world", "test"} {
		spec := SubAgentSpec{
			ID:     fmt.Sprintf("test-%d", i),
			Prompt: "echo " + msg,
			Budget: 1,
		}
		if err := pool.Spawn(spec); err != nil {
			t.Fatalf("spawn: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := pool.Collect(ctx)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Errorf("sub-agent %s failed: %v", r.ID, r.Err)
		}
	}
}

func TestSubAgentPoolDuplicateID(t *testing.T) {
	pool := NewSubAgentPool()
	spec := SubAgentSpec{ID: "dup", Prompt: "echo test"}
	pool.Spawn(spec)
	if err := pool.Spawn(spec); err == nil {
		t.Error("expected error for duplicate sub-agent ID")
	}
}

func TestSubAgentTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	ch := make(chan SubAgentResult, 1)
	_, err := AwaitSubAgent(ctx, ch)
	if err == nil {
		t.Error("expected timeout error")
	}
}
