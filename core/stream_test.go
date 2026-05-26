package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

func TestFauxStreaming(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "hello "},
			{Type: core.ProvContentDelta, ContentDelta: "world"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	events, err := core.StreamCall(context.Background(), prov, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}

	var result string
	for ev := range events {
		if ev.Type == core.StreamError {
			t.Fatal(ev.Err)
		}
		if ev.Type == core.StreamDelta {
			result += ev.Content
		}
	}
	if result != "hello world" {
		t.Errorf("expected 'hello world', got %q", result)
	}
}

func TestStreamChannelClose(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "done"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	events, err := core.StreamCall(context.Background(), prov, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}

	hasDone := false
	for ev := range events {
		if ev.Type == core.StreamDone {
			hasDone = true
		}
	}
	if !hasDone {
		t.Error("expected StreamDone event")
	}
}

func TestStreamDegrade(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "test"}},
	})
	events, err := core.StreamCall(context.Background(), prov, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var got string
	for ev := range events {
		if ev.Type == core.StreamDelta {
			got += ev.Content
		}
	}
	if got != "test" {
		t.Errorf("expected 'test', got %q", got)
	}
}

func TestStreamContextCancel(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvContentDelta, ContentDelta: "chunk1 "},
			{Type: core.ProvContentDelta, ContentDelta: "chunk2"},
		},
		TokensPerSecond: 1,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	events, err := core.StreamCall(ctx, prov, core.StreamRequest{})
	if err != nil {
		t.Fatal(err)
	}

	var result string
	for ev := range events {
		if ev.Type == core.StreamDelta {
			result += ev.Content
		}
	}
	_ = result
}

func TestStreamConcurrent(t *testing.T) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "call1"}},
	})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{{Type: core.ProvContentDelta, ContentDelta: "call2"}},
	})

	ch1, _ := core.StreamCall(context.Background(), prov, core.StreamRequest{})
	ch2, _ := core.StreamCall(context.Background(), prov, core.StreamRequest{})

	var r1, r2 string
	for ev := range ch1 {
		r1 += ev.Content
	}
	for ev := range ch2 {
		r2 += ev.Content
	}

	if r1+r2 != "call1call2" && r2+r1 != "call2call1" {
		t.Errorf("unexpected: r1=%q r2=%q", r1, r2)
	}
}
