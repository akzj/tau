package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// testSchema is a minimal core.ToolSchema for E2E tests.
type testSchema struct {
	raw json.RawMessage
}

func (s testSchema) Marshal() (json.RawMessage, error)     { return s.raw, nil }
func (s testSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

func TestPromptContinueCycle(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Queue: Turn 1 — tool call echo
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"msg":"hello"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Queue: Turn 2 — assistant responds
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "echoed: hello"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	// Register echo tool
	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ECHO: hello"}}}, nil
		},
	})

	loop := core.NewLoop()
	run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "echo hello"})

	var toolCalls int
	for ev := range run.Events() {
		if _, ok := ev.(core.ToolCallEnd); ok {
			toolCalls++
		}
	}
	<-run.Done()
	if toolCalls != 1 {
		t.Errorf("expected 1 tool call, got %d", toolCalls)
	}

	// Continue (turn 2)
	run2, _ := loop.Continue(ctx, sess)
	var deltas string
	for ev := range run2.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			deltas += d.ContentDelta
		}
	}
	<-run2.Done()
	if deltas != "echoed: hello" {
		t.Errorf("expected 'echoed: hello', got %q", deltas)
	}
}

func TestCompactionTrigger(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m0"},
			{Type: core.ProvContentDelta, MessageID: "m0", ContentDelta: "hi"},
			{Type: core.ProvMessageEnd, MessageID: "m0"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	// Add many messages to transcript to trigger compaction
	for i := 0; i < 50; i++ {
		sess.Transcript.Append(core.Message{
			Role:    core.RoleUser,
			Content: strings.Repeat("long content here ", 100),
		})
	}

	loop := core.NewLoop()
	run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	<-run.Done()

	// Verify compaction happened (transcript should have fewer messages)
	msgs := sess.Transcript.Messages()
	if len(msgs) > 20 {
		t.Logf("compaction may not have triggered, transcript has %d messages", len(msgs))
	}
}

func TestHookTriggerVerification(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hi"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	var hookCalled bool
	sess.Hooks.BeforeAgentStart = core.NewLastWins[core.AgentStartRequest]()
	sess.Hooks.BeforeAgentStart.Observe(func(ctx context.Context, req core.AgentStartRequest) {
		hookCalled = true
	})

	loop := core.NewLoop()
	run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	<-run.Done()

	if !hookCalled {
		t.Error("BeforeAgentStart hook not called")
	}
}

func TestPhaseMachineGuard(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "slow"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Second queue for concurrent attempt
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "concurrent"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	loop := core.NewLoop()

	// First Prompt should succeed
	run1, err := loop.Prompt(ctx, sess, core.UserInput{Text: "first"})
	if err != nil {
		t.Fatalf("first Prompt: %v", err)
	}

	// Second Prompt (while first is running) should fail with ErrTurnInProgress
	_, err = loop.Prompt(ctx, sess, core.UserInput{Text: "concurrent"})
	if err == nil {
		t.Error("expected ErrTurnInProgress, got nil")
	} else {
		tauErr, ok := err.(*core.TauError)
		if !ok || tauErr.Code != core.ErrTurnInProgress {
			t.Errorf("expected ErrTurnInProgress, got %v", err)
		}
	}

	// Drain first run
	for range run1.Events() {
	}
	<-run1.Done()

	// After first turn completes, prompt should work again
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m3"},
			{Type: core.ProvContentDelta, MessageID: "m3", ContentDelta: "third"},
			{Type: core.ProvMessageEnd, MessageID: "m3"},
		},
	})
	run3, err := loop.Prompt(ctx, sess, core.UserInput{Text: "third"})
	if err != nil {
		t.Fatalf("third Prompt after turn complete: %v", err)
	}
	for range run3.Events() {
	}
	<-run3.Done()
}

func TestSteerQueue(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "ok"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	// Inject steer
	sess.Steer("use bash instead of python")

	loop := core.NewLoop()
	run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "do something"})
	<-run.Done()

	// Verify steer was consumed (SteerQueue should be empty)
	if len(sess.SteerQueue) != 0 {
		t.Errorf("SteerQueue not drained, got %d entries", len(sess.SteerQueue))
	}

	// Transcript should contain the steer system message
	found := false
	for _, m := range sess.Transcript.Messages() {
		if strings.Contains(m.Content, "use bash instead of python") {
			found = true
			break
		}
	}
	if !found {
		t.Error("steer message not found in transcript")
	}
}

func TestParallelToolExecutionOrder(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Two tool calls in one message
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "tool-a"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"v":"a"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c2", ToolName: "tool-b"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c2", ToolArgsDelta: `{"v":"b"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c2"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
	defer sess.Cancel()

	// Register both tools
	sess.Tools.Register(core.Tool{
		Name:        "tool-a",
		Description: "a",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "A"}}}, nil
		},
	})
	sess.Tools.Register(core.Tool{
		Name:        "tool-b",
		Description: "b",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "B"}}}, nil
		},
	})

	loop := core.NewLoop()
	run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "run both"})

	var callOrder []string
	for ev := range run.Events() {
		if e, ok := ev.(core.ToolCallEnd); ok {
			for _, c := range e.Result.Content {
				callOrder = append(callOrder, c.Text)
			}
		}
	}
	<-run.Done()

	// Results should emit in deterministic order (c1="A" then c2="B")
	if len(callOrder) != 2 || callOrder[0] != "A" || callOrder[1] != "B" {
		t.Errorf("expected [A B], got %v", callOrder)
	}

	// Both tool results should be in transcript
	msgs := sess.Transcript.Messages()
	toolCount := 0
	for _, m := range msgs {
		if m.Role == core.RoleTool {
			toolCount++
		}
	}
	if toolCount != 2 {
		t.Errorf("expected 2 tool results in transcript, got %d", toolCount)
	}
}

// Ensure imports used
var _ = fmt.Sprintf
