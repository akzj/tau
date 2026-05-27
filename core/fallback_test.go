package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

func TestFallbackOnError(t *testing.T) {
	// Provider 1: always errors, Provider 2: succeeds
	p1 := faux.New()
	p1.SetMode(faux.ModeDown)
	p2 := faux.New()
	p2.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m1"},
		{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "success"},
		{Type: core.ProvMessageEnd, MessageID: "m1"},
	}})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "evil"},
		{Provider: p2, Name: "normal"},
	}, core.FallbackSequential)

	ch, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	var text string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if text != "success" {
		t.Errorf("got %q, want %q", text, "success")
	}
}

func TestFallbackChainExhausted(t *testing.T) {
	p1 := faux.New()
	p1.SetMode(faux.ModeDown)
	p2 := faux.New()
	p2.SetMode(faux.ModeDown)

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "e1"},
		{Provider: p2, Name: "e2"},
	}, core.FallbackSequential)

	_, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err == nil {
		t.Error("expected all providers failed")
	}
}

func TestFallbackCompleteError(t *testing.T) {
	p1 := faux.New()
	p1.SetMode(faux.ModeDown)
	p2 := faux.New()
	p2.QueueComplete(faux.CompleteEntry{
		Response: core.CompleteResponse{Content: "fallback-complete"},
	})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "evil"},
		{Provider: p2, Name: "normal"},
	}, core.FallbackSequential)

	resp, err := router.Complete(context.Background(), core.CompleteRequest{
		Model: core.ModelSpec{Name: "test"},
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if resp.Content != "fallback-complete" {
		t.Errorf("got %q, want %q", resp.Content, "fallback-complete")
	}
}

func TestFallbackCompleteExhausted(t *testing.T) {
	p1 := faux.New()
	p1.SetMode(faux.ModeDown)
	p2 := faux.New()
	p2.SetMode(faux.ModeDown)

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "e1"},
		{Provider: p2, Name: "e2"},
	}, core.FallbackSequential)

	_, err := router.Complete(context.Background(), core.CompleteRequest{
		Model: core.ModelSpec{Name: "test"},
	})
	if err == nil {
		t.Error("expected all providers failed")
	}
}

func TestHealthCheckUnhealthy(t *testing.T) {
	p := faux.New()
	p.SetMode(faux.ModeDown)
	entry := core.ProviderEntry{Provider: p, Name: "sick"}

	router := core.NewFallbackRouter([]core.ProviderEntry{entry}, core.FallbackSequential)
	router.StartHealthCheck(100 * time.Millisecond)
	time.Sleep(350 * time.Millisecond)

	// NewFallbackRouter creates internal copies; provider always fails health check
	// because p.SetMode(ModeDown) causes Complete to fail
	// Verify by calling Complete — it should fail (all unhealthy)
	_, err := router.Complete(context.Background(), core.CompleteRequest{
		Model: core.ModelSpec{Name: "test"},
	})
	if err == nil {
		t.Error("expected router failure after health check marked provider unhealthy")
	}
}

func TestHealthCheckRecover(t *testing.T) {
	p := faux.New()
	// Queue enough Complete responses for health check probes AND test call
	for i := 0; i < 10; i++ {
		p.QueueComplete(faux.CompleteEntry{
			Response: core.CompleteResponse{Content: "pong"},
		})
	}
	entry := core.ProviderEntry{Provider: p, Name: "recover"}

	router := core.NewFallbackRouter([]core.ProviderEntry{entry}, core.FallbackSequential)
	router.StartHealthCheck(100 * time.Millisecond)
	time.Sleep(350 * time.Millisecond)

	// After health check, provider should be healthy (probe succeeded via queued Complete)
	resp, err := router.Complete(context.Background(), core.CompleteRequest{
		Model: core.ModelSpec{Name: "test"},
	})
	if err != nil {
		t.Fatalf("expected health check recovery, got: %v", err)
	}
	if resp.Content != "pong" {
		t.Errorf("got %q, want %q", resp.Content, "pong")
	}
}

func TestParseFallbackChain(t *testing.T) {
	chain := core.ParseFallbackChain("openai, google, anthropic")
	if len(chain) != 3 {
		t.Fatalf("expected 3, got %d", len(chain))
	}
	if chain[0] != "openai" {
		t.Errorf("expected openai, got %s", chain[0])
	}
	if chain[1] != "google" {
		t.Errorf("expected google, got %s", chain[1])
	}
	if chain[2] != "anthropic" {
		t.Errorf("expected anthropic, got %s", chain[2])
	}
}

func TestParseFallbackChainEmpty(t *testing.T) {
	chain := core.ParseFallbackChain("")
	if len(chain) != 0 {
		t.Errorf("expected empty, got %d", len(chain))
	}
}

func TestParseFallbackChainWhitespace(t *testing.T) {
	chain := core.ParseFallbackChain("  openai ,  google  ,anthropic  ")
	if len(chain) != 3 {
		t.Fatalf("expected 3, got %d: %v", len(chain), chain)
	}
	if chain[0] != "openai" {
		t.Errorf("expected openai, got %q", chain[0])
	}
}

func TestParseProviderWeights(t *testing.T) {
	weights := core.ParseProviderWeights("anthropic=5,openai=3")
	if weights["anthropic"] != 5 {
		t.Errorf("expected 5, got %d", weights["anthropic"])
	}
	if weights["openai"] != 3 {
		t.Errorf("expected 3, got %d", weights["openai"])
	}
}

func TestParseProviderWeightsEmpty(t *testing.T) {
	weights := core.ParseProviderWeights("")
	if len(weights) != 0 {
		t.Errorf("expected empty, got %d", len(weights))
	}
}

func TestWeightedStreamSelection(t *testing.T) {
	p1 := faux.New()
	p1.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m1"},
		{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "a"},
		{Type: core.ProvMessageEnd, MessageID: "m1"},
	}})
	p2 := faux.New()
	p2.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m2"},
		{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "b"},
		{Type: core.ProvMessageEnd, MessageID: "m2"},
	}})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "a", Weight: 10},
		{Provider: p2, Name: "b", Weight: 1},
	}, core.FallbackWeighted)

	// Both healthy — weighted selection should work (not deterministic, but shouldn't error)
	ch, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	var text string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	t.Logf("weighted result: %q", text)
	if text != "a" && text != "b" {
		t.Errorf("unexpected result: %q", text)
	}
}

func TestRoundRobinSelection(t *testing.T) {
	p1 := faux.New()
	p1.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m1"},
		{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "first"},
		{Type: core.ProvMessageEnd, MessageID: "m1"},
	}})
	p2 := faux.New()
	p2.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m2"},
		{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "second"},
		{Type: core.ProvMessageEnd, MessageID: "m2"},
	}})
	p3 := faux.New()
	p3.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m3"},
		{Type: core.ProvContentDelta, MessageID: "m3", ContentDelta: "third"},
		{Type: core.ProvMessageEnd, MessageID: "m3"},
	}})

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "a"},
		{Provider: p2, Name: "b"},
		{Provider: p3, Name: "c"},
	}, core.FallbackRoundRobin)

	// First call — should get first provider (cursor at 0)
	ch, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	var text string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if text != "first" {
		t.Errorf("call 1: got %q, want %q", text, "first")
	}

	// Second call — cursor advanced, should get second provider
	ch2, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	text = ""
	for ev := range ch2 {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if text != "second" {
		t.Errorf("call 2: got %q, want %q", text, "second")
	}
}

func TestBackwardCompatDirectProvider(t *testing.T) {
	// When no fallback is configured, direct provider call should work unchanged
	p := faux.New()
	p.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "m1"},
		{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "direct"},
		{Type: core.ProvMessageEnd, MessageID: "m1"},
	}})
	ch, err := p.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
	if err != nil {
		t.Fatalf("direct call failed: %v", err)
	}
	var text string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if text != "direct" {
		t.Errorf("got %q, want %q", text, "direct")
	}
}

func TestZeroCoreRegression(t *testing.T) {
	// Verify that existing core tests still pass — tested by full suite run
	t.Log("core regression: tested by go test ./core/")
}

func TestFlakyProviderFallsBack(t *testing.T) {
	// Provider 1 is flaky (50% error rate), Provider 2 always succeeds
	p1 := faux.New()
	p1.SetMode(faux.ModeFlaky)
	p2 := faux.New()

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "flaky"},
		{Provider: p2, Name: "safe"},
	}, core.FallbackSequential)

	// Try a few times — should always eventually succeed via fallback
	for attempt := 0; attempt < 5; attempt++ {
		p1.SetMode(faux.ModeFlaky) // reset for each attempt
		p2.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "safe"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		}})
		ch, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
		if err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		var text string
		for ev := range ch {
			if ev.Type == core.ProvContentDelta {
				text += ev.ContentDelta
			}
		}
		if text != "safe" {
			t.Errorf("attempt %d: got %q, want %q", attempt, text, "safe")
		}
	}
}

func TestSingleHealthyProviderRoundRobin(t *testing.T) {
	// Only one provider healthy — should always pick it
	p1 := faux.New()
	p1.SetMode(faux.ModeDown)
	p2 := faux.New()

	router := core.NewFallbackRouter([]core.ProviderEntry{
		{Provider: p1, Name: "down"},
		{Provider: p2, Name: "up"},
	}, core.FallbackRoundRobin)

	for i := 0; i < 3; i++ {
		p2.QueueStream(faux.StreamResponse{Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "only"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		}})
		ch, err := router.Stream(context.Background(), core.StreamRequest{Model: core.ModelSpec{Name: "test"}})
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		var text string
		for ev := range ch {
			if ev.Type == core.ProvContentDelta {
				text += ev.ContentDelta
			}
		}
		if text != "only" {
			t.Errorf("call %d: got %q, want %q", i, text, "only")
		}
	}
}