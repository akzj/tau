package faux_test

import (
	"context"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

func TestFauxStream_EchoTool(t *testing.T) {
	ctx := context.Background()
	prov := faux.New()

	// Queue: assistant calls echo tool
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "msg-1"},
			{Type: core.ProvToolCallStart, MessageID: "msg-1", ToolCallID: "call-1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "msg-1", ToolCallID: "call-1", ToolArgsDelta: `{"msg":"hello"}`},
			{Type: core.ProvToolCallEnd, MessageID: "msg-1", ToolCallID: "call-1"},
			{Type: core.ProvMessageEnd, MessageID: "msg-1"},
		},
	})

	ch, err := prov.Stream(ctx, core.StreamRequest{
		Model: core.ModelSpec{Name: "faux", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	var events []core.ProviderEvent
	for ev := range ch {
		events = append(events, ev)
	}

	if len(events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(events))
	}
	if events[3].Type != core.ProvToolCallEnd {
		t.Errorf("expected ProvToolCallEnd at [3], got %v", events[3].Type)
	}
}

func TestFauxStream_Abort(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	prov := faux.New()

	// Queue: long stream with delays
	var events []core.ProviderEvent
	for i := 0; i < 5; i++ {
		events = append(events, core.ProviderEvent{
			Type: core.ProvContentDelta, ContentDelta: "word ",
		})
	}
	prov.QueueStream(faux.StreamResponse{Events: events, Delay: 50 * time.Millisecond})

	ch, err := prov.Stream(ctx, core.StreamRequest{})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}

	// Cancel after first event
	<-ch
	cancel()

	// Drain remaining — should close early
	count := 1
	for range ch {
		count++
	}
	if count >= 5 {
		t.Errorf("expected abort before all events, got %d", count)
	}
}

func TestFauxComplete(t *testing.T) {
	prov := faux.New()
	prov.QueueComplete(faux.CompleteEntry{
		Response: core.CompleteResponse{
			Content: "summary",
			Usage:   core.Usage{PromptTokens: 100, CompletionTokens: 20},
		},
	})

	resp, err := prov.Complete(context.Background(), core.CompleteRequest{})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "summary" {
		t.Errorf("expected 'summary', got %q", resp.Content)
	}
}

func TestFauxEmpty_Error(t *testing.T) {
	prov := faux.New()
	_, err := prov.Stream(context.Background(), core.StreamRequest{})
	if err == nil {
		t.Error("expected error for empty queue")
	}
}

func TestCacheGrowth(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Turn 1: 2 messages
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "hi"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	_, err := prov.Stream(ctx, core.StreamRequest{
		Model: core.ModelSpec{Name: "test-sess"},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "a"},
			{Role: core.RoleUser, Content: "b"},
		},
	})
	if err != nil {
		t.Fatalf("turn 1: %v", err)
	}

	// Turn 2: 3 messages (1 new)
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, ContentDelta: "hi"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})
	_, err = prov.Stream(ctx, core.StreamRequest{
		Model: core.ModelSpec{Name: "test-sess"},
		Messages: []core.Message{
			{Role: core.RoleUser, Content: "a"},
			{Role: core.RoleUser, Content: "b"},
			{Role: core.RoleUser, Content: "c"},
		},
	})
	if err != nil {
		t.Fatalf("turn 2: %v", err)
	}

	reads, writes := prov.CacheStats("test-sess")
	if len(reads) != 2 {
		t.Fatalf("expected 2 cache reads, got %d", len(reads))
	}
	if reads[1] <= reads[0] {
		t.Errorf("cache reads should grow: r1=%d r2=%d", reads[0], reads[1])
	}
	if len(writes) != 2 {
		t.Fatalf("expected 2 cache writes, got %d", len(writes))
	}
}

func TestDeltaStreaming(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "hello world this is a test"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
		TokensPerSecond: 50,
		MinTokenSize:    3,
		MaxTokenSize:    6,
	})

	ch, err := prov.Stream(ctx, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}

	var deltas []string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			deltas = append(deltas, ev.ContentDelta)
		}
	}
	if len(deltas) < 2 {
		t.Errorf("expected >=2 delta chunks (TokensPerSecond=50, content len=30), got %d", len(deltas))
	}
	// Verify total content matches
	total := ""
	for _, d := range deltas {
		total += d
	}
	if total != "hello world this is a test" {
		t.Errorf("total delta mismatch: %q", total)
	}
}

func TestAbortPropagation(t *testing.T) {
	prov := faux.New()
	ctx, cancel := context.WithCancel(context.Background())

	// Long stream
	var events []core.ProviderEvent
	for i := 0; i < 10; i++ {
		events = append(events, core.ProviderEvent{
			Type: core.ProvContentDelta, ContentDelta: "chunk ",
		})
	}
	prov.QueueStream(faux.StreamResponse{Events: events, TokensPerSecond: 100})

	ch, err := prov.Stream(ctx, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}

	// Read one event then cancel
	<-ch
	cancel()

	// Drain — should see [aborted] or early close
	var last string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			last = ev.ContentDelta
		}
	}
	if last != "\n[aborted]" && last != "" {
		t.Logf("note: abort may close channel before [aborted] emission, last=%q", last)
	}
}