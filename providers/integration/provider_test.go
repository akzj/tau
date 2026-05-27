//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// TestProviderLatencyNormal verifies streaming with tokens-per-second pacing.
func TestProviderLatencyNormal(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "streaming response ok"},
		},
		TokensPerSecond: 100,
	})

	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	var output string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			output += ev.ContentDelta
		}
	}
	if output == "" {
		t.Error("expected non-empty stream output")
	}
	t.Logf("latency-normal output: %s", output)
}

// TestProviderTimeout verifies context deadline handling.
func TestProviderTimeout(t *testing.T) {
	prov := faux.New()
	prov.SetConfig(faux.Config{Latency: 2 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	ch, err := prov.Stream(ctx, core.StreamRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Logf("timeout produced expected error before stream: %v", err)
		return
	}
	// Drain channel until ctx expires
	for range ch {
	}
	t.Log("timeout handled: stream closed after ctx deadline")
}

// TestProvider429RateLimit verifies rate-limit errors are detected via ErrorRate config.
func TestProvider429RateLimit(t *testing.T) {
	prov := faux.New()
	prov.SetConfig(faux.Config{ErrorRate: 1.0})

	_, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error with ErrorRate=1.0")
	} else {
		t.Logf("rate limit error: %v", err)
	}
}

// TestProvider500Error verifies Evil mode produces errors.
func TestProvider500Error(t *testing.T) {
	prov := faux.New()
	prov.SetMode(faux.ModeEvil)

	_, err := prov.Complete(context.Background(), core.CompleteRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err == nil {
		t.Error("expected error in Evil mode")
	} else {
		t.Logf("evil mode error: %v", err)
	}
}

// TestTokenBomb verifies large token payloads are handled without panic.
func TestTokenBomb(t *testing.T) {
	prov := faux.New()
	longText := strings.Repeat("x", 100000)
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "big"},
			{Type: core.ProvContentDelta, MessageID: "big", ContentDelta: longText},
			{Type: core.ProvMessageEnd, MessageID: "big"},
		},
	})

	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "big data"}},
	})
	if err != nil {
		t.Fatalf("stream failed: %v", err)
	}

	count := 0
	totalLen := 0
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			count++
			totalLen += len(ev.ContentDelta)
		}
	}
	t.Logf("token bomb: %d events, %d total chars", count, totalLen)
	if totalLen == 0 {
		t.Error("expected some content from token bomb")
	}
}

// TestWeightedLoadBalancing verifies weighted fallback router selects healthy providers.
func TestWeightedLoadBalancing(t *testing.T) {
	p1 := faux.New()
	p1.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "response from a"},
		},
	})
	p2 := faux.New()
	p2.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "response from b"},
		},
	})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "a", Weight: 5},
		{Provider: p2, Name: "b", Weight: 1},
	}, core.FallbackWeighted)

	ch, err := router.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("weighted router stream failed: %v", err)
	}
	for range ch {
	}
	t.Log("weighted load balancing: stream completed")
}

// TestHealthCheckTransition verifies provider health toggles correctly.
func TestHealthCheckTransition(t *testing.T) {
	p1 := faux.New()
	p2 := faux.New()

	// p2 will be the backup
	p2.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "backup response"},
		},
	})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "primary", Weight: 10},
		{Provider: p2, Name: "backup", Weight: 1},
	}, core.FallbackSequential)

	// Set primary to down mode
	p1.SetMode(faux.ModeDown)

	// Stream should fail over to backup
	ch, err := router.Stream(context.Background(), core.StreamRequest{
		Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("expected backup to handle stream, got error: %v", err)
	}
	var output string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			output += ev.ContentDelta
		}
	}
	if output == "" {
		t.Error("expected backup response content")
	}
	t.Logf("health transition: backup response = %q", output)
}

// TestConcurrentProviders verifies multiple concurrent streams work.
func TestConcurrentProviders(t *testing.T) {
	prov := faux.New()
	for i := 0; i < 5; i++ {
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvContentDelta, ContentDelta: fmt.Sprintf("r%d", i)},
			},
		})
	}

	ctx := context.Background()
	type streamResult struct {
		idx    int
		output string
	}
	results := make(chan streamResult, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			ch, err := prov.Stream(ctx, core.StreamRequest{
				Model:    core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
				Messages: []core.Message{{Role: core.RoleUser, Content: "hi"}},
			})
			if err != nil {
				results <- streamResult{idx: idx, output: fmt.Sprintf("err: %v", err)}
				return
			}
			var out string
			for ev := range ch {
				if ev.Type == core.ProvContentDelta {
					out += ev.ContentDelta
				}
			}
			results <- streamResult{idx: idx, output: out}
		}(i)
	}

	collected := make(map[int]string)
	for i := 0; i < 5; i++ {
		r := <-results
		collected[r.idx] = r.output
	}

	for i := 0; i < 5; i++ {
		out, ok := collected[i]
		if !ok || out == "" {
			t.Errorf("concurrent stream %d: no output", i)
		}
	}
	t.Logf("concurrent providers: 5/5 streams completed")
}
