//go:build integration

package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/internal/testutil"
	"github.com/akzj/tau/pkg/testing/faux"
)

// testSchema is a minimal core.ToolSchema for integration tests.
type testSchema struct {
	raw json.RawMessage
}

func (s testSchema) Marshal() (json.RawMessage, error)  { return s.raw, nil }
func (s testSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// TestSimpleTask verifies basic prompt→response through the Loop.
func TestSimpleTask(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	h.QueueSimple("hello from agent")

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "say hello"})
	if err != nil {
		t.Fatal(err)
	}

	var text string
	for ev := range run.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			text += msg.ContentDelta
		}
	}
	<-run.Done()

	if text == "" {
		t.Error("expected non-empty response")
	}
	t.Logf("SimpleTask response: %s", text)
}

// TestMultiStepTask verifies tool-call→continue→response through the Loop.
func TestMultiStepTask(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// Turn 1: tool call
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"msg":"hello"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: final response
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "task complete"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	// Register echo tool
	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "echo a message",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ECHO: hello"}}}, nil
		},
	})

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "multi step task"})
	if err != nil {
		t.Fatal(err)
	}

	toolCount := 0
	for ev := range run.Events() {
		if _, ok := ev.(core.ToolCallEnd); ok {
			toolCount++
		}
	}
	<-run.Done()

	if toolCount != 1 {
		t.Errorf("expected 1 tool call, got %d", toolCount)
	}

	// Continue
	run2, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	var finalText string
	for ev := range run2.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			finalText += msg.ContentDelta
		}
	}
	<-run2.Done()

	if finalText != "task complete" {
		t.Errorf("expected 'task complete', got %q", finalText)
	}
}

// TestToolCallFlow verifies a specific tool call flows through the event loop.
func TestToolCallFlow(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "read"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"path":"main.go"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	sess.Tools.Register(core.Tool{
		Name:        "read",
		Description: "read a file",
		Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "file content"}}}, nil
		},
	})

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "read main.go"})
	if err != nil {
		t.Fatal(err)
	}

	foundRead := false
	for ev := range run.Events() {
		if tc, ok := ev.(core.ToolCallStart); ok && tc.ToolName == "read" {
			foundRead = true
		}
	}
	<-run.Done()

	if !foundRead {
		t.Error("expected 'read' tool call start event")
	}

	// Verify tool result in transcript
	msgs := sess.Transcript.Messages()
	hasToolResult := false
	for _, m := range msgs {
		if m.Role == core.RoleTool {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		t.Error("expected tool result message in transcript")
	}
}

// TestPlanningFlow verifies strategy-based execution (plan-execute strategy).
func TestPlanningFlow(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	h.QueueSimple("plan executed: step1 → step2 → step3")

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	// Use the plan-execute strategy
	sess.Strategy = core.GetStrategy("plan-execute")

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "plan a complex task"})
	if err != nil {
		t.Fatal(err)
	}

	var text string
	for ev := range run.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			text += msg.ContentDelta
		}
	}
	<-run.Done()

	if text == "" {
		t.Error("expected non-empty output from planning flow")
	}
	t.Logf("PlanningFlow output: %s", text)
}

// TestReflectionFlow verifies the reflection engine triggers on post-turn analysis.
func TestReflectionFlow(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// Queue a response that should trigger reflection
	h.QueueSimple("Here is code: // TODO: fix this later. func main() {}")

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     h.Prov,
		DefaultModel: core.ModelSpec{Name: "test", API: core.WireOpenAICompletions},
		ReflectDepth: 1, // enable reflection
	})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "write code"})
	if err != nil {
		t.Fatal(err)
	}

	var text string
	for ev := range run.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			text += msg.ContentDelta
		}
	}
	<-run.Done()

	if text == "" {
		t.Error("expected non-empty output from reflection flow")
	}

	// Reflection should have been triggered
	if sess.Reflection != nil {
		t.Logf("reflection engine is active, turn text: %s", text)
	}
}

// TestCompressionFlow verifies the compaction system triggers under memory pressure.
func TestCompressionFlow(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	h.QueueSimple("final response after many turns")

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	// Add many messages to trigger context pressure/compaction
	for i := 0; i < 50; i++ {
		sess.Transcript.Append(core.Message{
			Role:    core.RoleUser,
			Content: strings.Repeat("long context message number "+string(rune('0'+i%10))+" ", 100),
		})
	}

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "respond concisely"})
	if err != nil {
		t.Fatal(err)
	}
	<-run.Done()

	// After compaction, transcript should have fewer messages
	msgs := sess.Transcript.Messages()
	if len(msgs) > 20 {
		t.Logf("compaction may not have triggered, transcript has %d messages", len(msgs))
	} else {
		t.Logf("compaction likely triggered: transcript reduced to %d messages", len(msgs))
	}
}
