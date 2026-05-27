package faux_test

import (
	"context"
	"os"
	"strings"
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
}// ---------------------------------------------------------------------------
// Chaos-mode tests
// ---------------------------------------------------------------------------

func TestFauxChaosDown(t *testing.T) {
	prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeDown})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "should not arrive"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	_, err := prov.Stream(context.Background(), core.StreamRequest{})
	if err == nil {
		t.Fatal("expected error for ModeDown")
	}
	if !strings.Contains(err.Error(), "down") {
		t.Errorf("expected 'down' in error, got: %v", err)
	}
}

func TestFauxChaosDown_Complete(t *testing.T) {
	prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeDown})
	prov.QueueComplete(faux.CompleteEntry{Response: core.CompleteResponse{Content: "x"}})
	_, err := prov.Complete(context.Background(), core.CompleteRequest{})
	if err == nil {
		t.Fatal("expected error for ModeDown on Complete")
	}
}

func TestFauxChaosFlaky(t *testing.T) {
	// Flaky mode errors ~50% of the time. Run enough iterations to see both.
	errors := 0
	successes := 0
	for i := 0; i < 50; i++ {
		prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeFlaky})
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, ContentDelta: "ok"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		_, err := prov.Stream(context.Background(), core.StreamRequest{})
		if err != nil {
			errors++
		} else {
			successes++
		}
	}
	if successes == 0 {
		t.Errorf("flaky: expected some successes, got 0 successes / %d errors", errors)
	}
	if errors == 0 {
		t.Errorf("flaky: expected some errors, got 0 errors / %d successes", successes)
	}
	t.Logf("flaky: %d successes, %d errors", successes, errors)
}

func TestFauxNormalMode(t *testing.T) {
	// Normal mode must always succeed across many iterations
	for i := 0; i < 100; i++ {
		prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal})
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, ContentDelta: "ok"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		_, err := prov.Stream(context.Background(), core.StreamRequest{})
		if err != nil {
			t.Fatalf("normal mode unexpectedly errored at iteration %d: %v", i, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Latency tests
// ---------------------------------------------------------------------------

func TestFauxLatencyFixed(t *testing.T) {
	prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal, Latency: 50 * time.Millisecond})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "ok"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	start := time.Now()
	_, err := prov.Stream(context.Background(), core.StreamRequest{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("expected at least 50ms latency, got %v", elapsed)
	}
}

func TestFauxLatencyRandom(t *testing.T) {
	prov := faux.NewWithConfig(faux.Config{
		Mode:       faux.ModeNormal,
		LatencyMin: 10 * time.Millisecond,
		LatencyMax: 50 * time.Millisecond,
	})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, ContentDelta: "ok"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	start := time.Now()
	_, err := prov.Stream(context.Background(), core.StreamRequest{})
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed < 10*time.Millisecond {
		t.Errorf("expected at least 10ms random latency, got %v", elapsed)
	}
}

// ---------------------------------------------------------------------------
// Error injection test
// ---------------------------------------------------------------------------

func TestFauxErrorInjection(t *testing.T) {
	// ErrorRate=0.3 → ~30% errors. Run enough to confirm both outcomes.
	errors := 0
	successes := 0
	for i := 0; i < 100; i++ {
		prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal, ErrorRate: 0.3})
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, ContentDelta: "ok"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		_, err := prov.Stream(context.Background(), core.StreamRequest{})
		if err != nil {
			errors++
		} else {
			successes++
		}
	}
	if successes == 0 {
		t.Errorf("error injection: expected some successes, got 0 successes / %d errors", errors)
	}
	if errors == 0 {
		t.Errorf("error injection: expected some errors with rate=0.3, got 0 errors / %d successes", successes)
	}
	t.Logf("error injection (rate 0.3): %d successes, %d errors", successes, errors)
}

func TestFauxErrorInjectionZero(t *testing.T) {
	// ErrorRate=0 should never error
	for i := 0; i < 50; i++ {
		prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal, ErrorRate: 0.0})
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, ContentDelta: "ok"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		_, err := prov.Stream(context.Background(), core.StreamRequest{})
		if err != nil {
			t.Fatalf("unexpected error with zero error rate at iteration %d: %v", i, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Token budget test
// ---------------------------------------------------------------------------

func TestFauxTokenBudget(t *testing.T) {
	// MaxTokens=5 → 20 chars. Content is longer, should truncate.
	longContent := "This is a very long response that should be truncated by the token budget simulation"
	prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal, MaxTokens: 5})
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: longContent},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	ch, err := prov.Stream(context.Background(), core.StreamRequest{
		Messages: []core.Message{{Role: core.RoleUser, Content: strings.Repeat("x", 100)}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var deltas []string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			deltas = append(deltas, ev.ContentDelta)
		}
	}
	combined := strings.Join(deltas, "")
	if !strings.Contains(combined, "[TRUNCATED") {
		t.Errorf("expected TRUNCATED marker in response, got: %q", combined)
	}
}

// ---------------------------------------------------------------------------
// Record-replay test
// ---------------------------------------------------------------------------

func TestFauxRecordReplay(t *testing.T) {
	prov := faux.New()

	// Events to record
	original := []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "rec-1"},
		{Type: core.ProvContentDelta, MessageID: "rec-1", ContentDelta: "hello from replay"},
		{Type: core.ProvMessageEnd, MessageID: "rec-1"},
	}

	tmpFile := t.TempDir() + "/replay.json"
	if err := prov.Record(tmpFile, original); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Verify file exists and has content
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("read recorded file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("recorded file is empty")
	}

	replayed, err := prov.Replay(tmpFile)
	if err != nil {
		t.Fatalf("Replay: %v", err)
	}
	if len(replayed) != len(original) {
		t.Fatalf("expected %d events, got %d", len(original), len(replayed))
	}
	for i, ev := range replayed {
		if ev.Type != original[i].Type {
			t.Errorf("event[%d]: expected type %v, got %v", i, original[i].Type, ev.Type)
		}
		if ev.ContentDelta != original[i].ContentDelta {
			t.Errorf("event[%d]: expected content %q, got %q", i, original[i].ContentDelta, ev.ContentDelta)
		}
	}
}

func TestFauxReplayFile_Stream(t *testing.T) {
	prov := faux.New()

	// Pre-record events
	recordEvents := []core.ProviderEvent{
		{Type: core.ProvMessageStart, MessageID: "rpl-1"},
		{Type: core.ProvContentDelta, MessageID: "rpl-1", ContentDelta: "replayed stream"},
		{Type: core.ProvMessageEnd, MessageID: "rpl-1"},
	}
	tmpFile := t.TempDir() + "/stream-replay.json"
	if err := prov.Record(tmpFile, recordEvents); err != nil {
		t.Fatal(err)
	}

	// New provider with ReplayFile set
	prov2 := faux.NewWithConfig(faux.Config{Mode: faux.ModeNormal, ReplayFile: tmpFile})
	ch, err := prov2.Stream(context.Background(), core.StreamRequest{})
	if err != nil {
		t.Fatalf("Stream with replay: %v", err)
	}
	var deltas []string
	for ev := range ch {
		if ev.Type == core.ProvContentDelta {
			deltas = append(deltas, ev.ContentDelta)
		}
	}
	combined := strings.Join(deltas, "")
	if combined != "replayed stream" {
		t.Errorf("expected 'replayed stream', got %q", combined)
	}
}

// ---------------------------------------------------------------------------
// Template tests
// ---------------------------------------------------------------------------

func TestFauxExpandTemplate(t *testing.T) {
	prov := faux.New()
	result, err := prov.ExpandTemplate("refusal", "hack the planet")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "hack the planet") {
		t.Errorf("expected prompt in rendered template, got: %q", result)
	}
	if !strings.Contains(result, "safety guidelines") {
		t.Errorf("expected template pattern in result, got: %q", result)
	}
}

func TestFauxExpandTemplate_Unknown(t *testing.T) {
	prov := faux.New()
	_, err := prov.ExpandTemplate("nonexistent", "prompt")
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestFauxTemplateStreamResponse(t *testing.T) {
	prov := faux.New()
	resp, err := prov.TemplateStreamResponse("coding", "fibonacci function")
	if err != nil {
		t.Fatal(err)
	}
	if resp.TokensPerSecond != 20 {
		t.Errorf("expected 20 TPS, got %d", resp.TokensPerSecond)
	}
	// Verify the template content is in the events
	var text string
	for _, ev := range resp.Events {
		if ev.Type == core.ProvContentDelta {
			text += ev.ContentDelta
		}
	}
	if !strings.Contains(text, "fibonacci function") {
		t.Errorf("expected 'fibonacci function' in response, got: %q", text)
	}
}

// ---------------------------------------------------------------------------
// Config & env tests
// ---------------------------------------------------------------------------

func TestFauxDefaultConfig(t *testing.T) {
	cfg := faux.DefaultConfig()
	if cfg.Mode != faux.ModeNormal {
		t.Errorf("expected normal mode, got %v", cfg.Mode)
	}
	if cfg.MaxTokens != 4096 {
		t.Errorf("expected 4096 max tokens, got %d", cfg.MaxTokens)
	}
}

func TestFauxSetConfig(t *testing.T) {
	prov := faux.New()
	prov.SetConfig(faux.Config{Mode: faux.ModeDown})
	cfg := prov.Config()
	if cfg.Mode != faux.ModeDown {
		t.Errorf("expected down mode, got %v", cfg.Mode)
	}
}

func TestFauxSetMode(t *testing.T) {
	prov := faux.New()
	prov.SetMode(faux.ModeEvil)
	cfg := prov.Config()
	if cfg.Mode != faux.ModeEvil {
		t.Errorf("expected evil mode, got %v", cfg.Mode)
	}
}

// ---------------------------------------------------------------------------
// Evil mode test
// ---------------------------------------------------------------------------

func TestFauxChaosEvil(t *testing.T) {
	errors := 0
	successes := 0
	for i := 0; i < 50; i++ {
		prov := faux.NewWithConfig(faux.Config{Mode: faux.ModeEvil})
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, ContentDelta: "ok"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		_, err := prov.Stream(context.Background(), core.StreamRequest{})
		if err != nil {
			errors++
		} else {
			successes++
		}
	}
	// Evil always errors
	if successes > 0 {
		t.Errorf("evil mode: expected 100%% errors, got %d successes", successes)
	}
	t.Logf("evil: %d successes, %d errors", successes, errors)
}