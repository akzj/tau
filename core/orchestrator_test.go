package core_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/akzj/tau/pkg/testing/faux"
	"github.com/akzj/tau/core"
)

func TestAgentPoolSingle(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "hello"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Delegate(context.Background(), []core.DelegationTask{
		{ID: "t1", Prompt: "hi", MaxTurns: 2, Timeout: 5 * time.Second},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].TaskID != "t1" {
		t.Errorf("expected t1, got %s", results[0].TaskID)
	}
	if results[0].Response != "hello" {
		t.Errorf("expected hello, got %q", results[0].Response)
	}
}

func TestAgentPoolThreeParallel(t *testing.T) {
	prov := faux.New()
	for i := 0; i < 3; i++ {
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvContentDelta, ContentDelta: fmt.Sprintf("result-%d", i)},
			},
		})
	}
	pool := core.NewAgentPool(3, prov, core.ModelSpec{Name: "test", API: ""})
	tasks := []core.DelegationTask{
		{ID: "t1", Prompt: "a", MaxTurns: 2},
		{ID: "t2", Prompt: "b", MaxTurns: 2},
		{ID: "t3", Prompt: "c", MaxTurns: 2},
	}
	start := time.Now()
	results := pool.Delegate(context.Background(), tasks)
	elapsed := time.Since(start)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if elapsed > 2*time.Second {
		t.Errorf("too slow: %s", elapsed)
	}
	for _, r := range results {
		if r.Error != "" {
			t.Errorf("unexpected error for %s: %s", r.TaskID, r.Error)
		}
	}
}

func TestAgentPoolTimeout(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events:          []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "slow"}},
		TokensPerSecond: 1,
	})
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Delegate(context.Background(), []core.DelegationTask{
		{ID: "t1", Prompt: "hi", MaxTurns: 1, Timeout: 1 * time.Nanosecond},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	// Should have error or empty response (timeout)
}

func TestAgentPoolOneFailOthersContinue(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvError, Err: fmt.Errorf("provider error")}},
	})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "success"}},
	})
	pool := core.NewAgentPool(2, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Delegate(context.Background(), []core.DelegationTask{
		{ID: "t1", Prompt: "fail", MaxTurns: 1},
		{ID: "t2", Prompt: "ok", MaxTurns: 1},
	})
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	failFound := false
	okFound := false
	for _, r := range results {
		if r.Error != "" {
			failFound = true
		}
		if r.Response == "success" {
			okFound = true
		}
	}
	if !failFound {
		t.Error("expected one failing task")
	}
	if !okFound {
		t.Error("expected one successful task")
	}
}

func TestAgentPoolCleanup(t *testing.T) {
	prov := faux.New()
	for i := 0; i < 3; i++ {
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "done"}},
		})
	}
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	for i := 0; i < 3; i++ {
		results := pool.Delegate(context.Background(), []core.DelegationTask{
			{ID: fmt.Sprintf("t%d", i), Prompt: "p", MaxTurns: 1},
		})
		if len(results) != 1 {
			t.Errorf("run %d: expected 1, got %d", i, len(results))
		}
	}
}

func TestAgentPoolZeroOverhead(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "zero"}},
	})
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Delegate(context.Background(), []core.DelegationTask{
		{ID: "t1", Prompt: "test", MaxTurns: 1},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
	if results[0].Response != "zero" {
		t.Errorf("got %q", results[0].Response)
	}
}

func TestAgentPoolMultipleSizes(t *testing.T) {
	for _, size := range []int{1, 3, 5} {
		prov := faux.New()
		for i := 0; i < size; i++ {
			prov.QueueStream(faux.StreamResponse{
				Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: fmt.Sprintf("size-%d", size)}},
			})
		}
		pool := core.NewAgentPool(size, prov, core.ModelSpec{Name: "test", API: ""})
		tasks := make([]core.DelegationTask, size)
		for i := range tasks {
			tasks[i] = core.DelegationTask{ID: fmt.Sprintf("t%d", i), Prompt: "p", MaxTurns: 1}
		}
		results := pool.Delegate(context.Background(), tasks)
		if len(results) != size {
			t.Errorf("size %d: expected %d, got %d", size, size, len(results))
		}
	}
}

func TestAgentPoolCancelContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pool := core.NewAgentPool(1, nil, core.ModelSpec{})
	results := pool.Delegate(ctx, []core.DelegationTask{
		{ID: "t1", Prompt: "test", MaxTurns: 1},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestAgentPoolEmptyTasks(t *testing.T) {
	pool := core.NewAgentPool(1, nil, core.ModelSpec{})
	results := pool.Delegate(context.Background(), nil)
	if len(results) != 0 {
		t.Error("expected empty results")
	}
}

func TestAgentPoolTaskID(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "resp"}},
	})
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Delegate(context.Background(), []core.DelegationTask{
		{ID: "unique-id-123", Prompt: "test", MaxTurns: 1},
	})
	if results[0].TaskID != "unique-id-123" {
		t.Errorf("expected id preserved, got %s", results[0].TaskID)
	}
}

func TestAgentPoolCollect(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "collected"}},
	})
	pool := core.NewAgentPool(1, prov, core.ModelSpec{Name: "test", API: ""})
	results := pool.Collect(context.Background(), []core.DelegationTask{
		{ID: "x", Prompt: "p", MaxTurns: 1},
	})
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
}

func TestAgentPoolSize(t *testing.T) {
	pool := core.NewAgentPool(5, nil, core.ModelSpec{})
	if pool.Size() != 5 {
		t.Errorf("expected size 5, got %d", pool.Size())
	}
	pool2 := core.NewAgentPool(0, nil, core.ModelSpec{})
	if pool2.Size() != 1 {
		t.Errorf("zero defaults to 1, got %d", pool2.Size())
	}
}
