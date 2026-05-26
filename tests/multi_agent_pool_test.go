//go:build e2e

package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/akzj/tau/core"
)

func TestMultiAgentPoolLimits(t *testing.T) {
	if os.Getenv("TAU_BIN") == "" {
		t.Skip("TAU_BIN not set — required for sub-agent spawning")
	}

	pool := core.NewSubAgentPool()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		spec := core.SubAgentSpec{
			ID:     fmt.Sprintf("limit-test-%d", i),
			Prompt: fmt.Sprintf("echo 'agent %d done'", i),
			Budget: 1,
			Tools:  nil,
		}
		if err := pool.Spawn(spec); err != nil {
			t.Fatalf("spawn %s: %v", spec.ID, err)
		}
	}

	results, err := pool.Collect(ctx)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(results) != 5 {
		t.Errorf("expected 5 results, got %d", len(results))
	}

	for _, r := range results {
		if r.Err != nil {
			t.Logf("[%s] error: %v", r.ID, r.Err)
		}
	}

	t.Logf("All %d pool agents completed", len(results))
}
