package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/coding/tools"
	"github.com/akzj/tau/pkg/testing/faux"
)

// toolSchema wraps a raw schema for testing convenience.
func toolSchema(raw string) core.ToolSchema {
	return tools.Schema{Raw: json.RawMessage(raw)}
}

func TestMultiTurnToolE2E(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Turn 1: echo tool call
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"msg":"test"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: assistant response
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "echoed test"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      toolSchema(`{"type":"object","properties":{"msg":{"type":"string"}},"required":["msg"]}`),
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ECHO: test"}}}, nil
		},
	})

	loop := core.NewLoop()

	// Turn 1: prompt → tool call → tool execute
	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "echo test"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var toolEndCount int
	for ev := range run.Events() {
		if _, ok := ev.(core.ToolCallEnd); ok {
			toolEndCount++
		}
	}
	<-run.Done()
	if toolEndCount != 1 {
		t.Errorf("expected 1 ToolCallEnd, got %d", toolEndCount)
	}

	// Turn 2: continue (tool result → assistant reply)
	run2, err := loop.Continue(ctx, sess)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	var content string
	for ev := range run2.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			content += d.ContentDelta
		}
	}
	<-run2.Done()
	if content != "echoed test" {
		t.Errorf("expected 'echoed test', got %q", content)
	}

	// Verify transcript has expected messages
	msgs := sess.Transcript.Messages()
	if len(msgs) < 4 {
		t.Errorf("expected >=4 messages, got %d", len(msgs))
	}
}

func TestStreamingChunkAccumulation(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Simulate streaming response with realistic deltas
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hello "},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "world "},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "from tau"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	loop := core.NewLoop()
	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var accumulated string
	for ev := range run.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			accumulated += d.ContentDelta
		}
	}
	<-run.Done()

	if accumulated != "hello world from tau" {
		t.Errorf("accumulation mismatch: %q", accumulated)
	}
}

func TestErrorInjectionRecovery(t *testing.T) {
	prov := faux.New()
	ctx := context.Background()

	// Turn 1: error
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvError, Err: fmt.Errorf("simulated fault")},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: recovery
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "recovered"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := core.NewSession(ctx, core.SessionOptions{
		Provider:     prov,
		DefaultModel: core.ModelSpec{Name: "test-model", API: core.WireOpenAICompletions},
	})
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	defer sess.Cancel()

	loop := core.NewLoop()

	run, err := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	var errorCount int
	for ev := range run.Events() {
		if _, ok := ev.(core.ErrorEvent); ok {
			errorCount++
		}
	}
	<-run.Done()
	if errorCount != 1 {
		t.Errorf("expected 1 error event, got %d", errorCount)
	}

	// Recover: next turn should work
	run2, err := loop.Continue(ctx, sess)
	if err != nil {
		t.Fatalf("Continue: %v", err)
	}
	var content string
	for ev := range run2.Events() {
		if d, ok := ev.(core.MessageDelta); ok {
			content += d.ContentDelta
		}
	}
	<-run2.Done()
	if content != "recovered" {
		t.Errorf("expected 'recovered', got %q", content)
	}
}

func TestWebSearchTool(t *testing.T) {
	tool := tools.WebSearchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"query": "golang"}, nil)
	if err != nil {
		t.Fatalf("web_search: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
}

func TestWebSearchEmptyQuery(t *testing.T) {
	tool := tools.WebSearchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"query": ""}, nil)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestWebFetchTool(t *testing.T) {
	tool := tools.WebFetchTool()
	result, err := tool.Execute(context.Background(), "c1", map[string]any{"url": "https://example.com"}, nil)
	if err != nil {
		t.Fatalf("web_fetch: %v", err)
	}
	if len(result.Content) == 0 {
		t.Error("expected content")
	}
	if result.Details["status"].(float64) != 200 {
		t.Error("expected 200 status")
	}
}

func TestWebFetchInvalidURL(t *testing.T) {
	tool := tools.WebFetchTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"url": "not-a-url"}, nil)
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

func TestTaskTrackerCreate(t *testing.T) {
	tools.GlobalTaskTracker = &tools.TaskTracker{}
	// Re-init map since TaskTracker can't be created from outside package
	// Reset by creating a fresh tracker via the struct (fields must be set)
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "fix bug"}, nil)
	if result.Content[0].Text != "Created task task-1: fix bug [pending]" {
		t.Errorf("unexpected result: %s", result.Content[0].Text)
	}
}

func TestTaskTrackerUpdateAndList(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	tool.Execute(context.Background(), "c1", map[string]any{"action": "create", "title": "task A"}, nil)
	tool.Execute(context.Background(), "c2", map[string]any{"action": "create", "title": "task B"}, nil)
	tool.Execute(context.Background(), "c3", map[string]any{"action": "update", "task_id": "task-1", "status": "completed"}, nil)
	result, _ := tool.Execute(context.Background(), "c4", map[string]any{"action": "list"}, nil)
	if !strings.Contains(result.Content[0].Text, "[✓]") {
		t.Error("completed task should have ✓ icon")
	}
}

func TestTaskTrackerEmptyList(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	result, _ := tool.Execute(context.Background(), "c1", map[string]any{"action": "list"}, nil)
	if result.Content[0].Text != "No tasks." {
		t.Errorf("expected 'No tasks.', got %q", result.Content[0].Text)
	}
}

func TestTaskTrackerUpdateNotFound(t *testing.T) {
	tools.ResetGlobalTaskTracker()
	tool := tools.TaskTrackerTool()
	_, err := tool.Execute(context.Background(), "c1", map[string]any{"action": "update", "task_id": "nonexistent", "status": "completed"}, nil)
	if err == nil {
		t.Error("expected error for nonexistent task")
	}
}
