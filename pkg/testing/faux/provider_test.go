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