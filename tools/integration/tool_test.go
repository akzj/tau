//go:build integration

package tool_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// ---------------------------------------------------------------------------
// Code pipeline: write → format → test → commit
// ---------------------------------------------------------------------------

func TestCodePipeline(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// Turn 1: tool call to write_file
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "write_file"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"path":"main.go","content":"package main"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: format_code
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvToolCallStart, MessageID: "m2", ToolCallID: "c2", ToolName: "format_code"},
			{Type: core.ProvToolCallDelta, MessageID: "m2", ToolCallID: "c2", ToolArgsDelta: `{"path":"main.go"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m2", ToolCallID: "c2"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})
	// Turn 3: run_test
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m3"},
			{Type: core.ProvToolCallStart, MessageID: "m3", ToolCallID: "c3", ToolName: "run_test"},
			{Type: core.ProvToolCallDelta, MessageID: "m3", ToolCallID: "c3", ToolArgsDelta: `{"target":"./..."}`},
			{Type: core.ProvToolCallEnd, MessageID: "m3", ToolCallID: "c3"},
			{Type: core.ProvMessageEnd, MessageID: "m3"},
		},
	})
	// Turn 4: final response
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m4"},
			{Type: core.ProvContentDelta, MessageID: "m4", ContentDelta: "code pipeline complete"},
			{Type: core.ProvMessageEnd, MessageID: "m4"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	registerPipelineTools(sess)

	// Turn 1: write_file
	run1, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "write, format, and test code"})
	if err != nil {
		t.Fatal(err)
	}
	for range run1.Events() {
	}
	<-run1.Done()

	// Turn 2: Continue → format_code
	run2, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	for range run2.Events() {
	}
	<-run2.Done()

	// Turn 3: Continue → run_test
	run3, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	for range run3.Events() {
	}
	<-run3.Done()

	// Turn 4: Continue → final response
	run4, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	var finalText string
	for ev := range run4.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			finalText += msg.ContentDelta
		}
	}
	<-run4.Done()

	// Verify tool calls happened across all turns
	msgs := sess.Transcript.Messages()
	toolCount := 0
	for _, m := range msgs {
		if m.Role == core.RoleTool {
			toolCount++
		}
	}
	if toolCount < 3 {
		t.Errorf("expected >=3 tool results in transcript, got %d", toolCount)
	}
	if finalText != "code pipeline complete" {
		t.Errorf("expected 'code pipeline complete', got %q", finalText)
	}
	t.Logf("code pipeline: %d tool calls, final: %q", toolCount, finalText)
}

// ---------------------------------------------------------------------------
// Git pipeline: diff → commit → log
// ---------------------------------------------------------------------------

func TestGitPipeline(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// Turn 1: git_diff
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "git_diff"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// Turn 2: git_commit
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvToolCallStart, MessageID: "m2", ToolCallID: "c2", ToolName: "git_commit"},
			{Type: core.ProvToolCallDelta, MessageID: "m2", ToolCallID: "c2", ToolArgsDelta: `{"message":"fix"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m2", ToolCallID: "c2"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})
	// Turn 3: git_log then final
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m3"},
			{Type: core.ProvContentDelta, MessageID: "m3", ContentDelta: "git pipeline done"},
			{Type: core.ProvMessageEnd, MessageID: "m3"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	registerGitTools(sess)

	// Turn 1: git_diff
	run1, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "diff, commit, log"})
	if err != nil {
		t.Fatal(err)
	}
	for range run1.Events() {
	}
	<-run1.Done()

	// Turn 2: Continue → git_commit
	run2, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	for range run2.Events() {
	}
	<-run2.Done()

	// Turn 3: Continue → final response
	run3, err := h.Loop.Continue(ctx, sess)
	if err != nil {
		t.Fatal(err)
	}
	var finalText string
	for ev := range run3.Events() {
		if msg, ok := ev.(core.MessageDelta); ok {
			finalText += msg.ContentDelta
		}
	}
	<-run3.Done()

	// Verify tool calls in transcript
	msgs := sess.Transcript.Messages()
	toolCount := 0
	for _, m := range msgs {
		if m.Role == core.RoleTool {
			toolCount++
		}
	}
	if toolCount < 2 {
		t.Errorf("expected >=2 tool results in transcript, got %d", toolCount)
	}
	if finalText != "git pipeline done" {
		t.Errorf("expected 'git pipeline done', got %q", finalText)
	}
	t.Logf("git pipeline: %d tool calls, final: %q", toolCount, finalText)
}

// ---------------------------------------------------------------------------
// Web pipeline: web_search → web_fetch
// ---------------------------------------------------------------------------

func TestWebPipeline(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// web_search
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "web_search"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"query":"golang"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// web_fetch then final
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "web pipeline done"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	registerWebTools(sess)

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "search and fetch"})
	if err != nil {
		t.Fatal(err)
	}

	var toolCalls []string
	for ev := range run.Events() {
		if tc, ok := ev.(core.ToolCallStart); ok {
			toolCalls = append(toolCalls, tc.ToolName)
		}
	}
	<-run.Done()

	if len(toolCalls) < 1 {
		t.Error("expected at least 1 tool call in web pipeline")
	}
	t.Logf("web pipeline tools called: %v", toolCalls)
}

// ---------------------------------------------------------------------------
// Data pipeline: json_query → csv_query
// ---------------------------------------------------------------------------

func TestDataPipeline(t *testing.T) {
	h := testutil.NewHarness(t)
	ctx := context.Background()

	// json_query
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "json_query"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"file":"data.json","query":".items"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})
	// csv_query then final
	h.Prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m2"},
			{Type: core.ProvContentDelta, MessageID: "m2", ContentDelta: "data pipeline done"},
			{Type: core.ProvMessageEnd, MessageID: "m2"},
		},
	})

	sess, err := h.NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Cancel()

	registerDataTools(sess)

	run, err := h.Loop.Prompt(ctx, sess, core.UserInput{Text: "query json data"})
	if err != nil {
		t.Fatal(err)
	}

	var toolCalls []string
	for ev := range run.Events() {
		if tc, ok := ev.(core.ToolCallStart); ok {
			toolCalls = append(toolCalls, tc.ToolName)
		}
	}
	<-run.Done()

	if len(toolCalls) < 1 {
		t.Error("expected at least 1 tool call in data pipeline")
	}
	t.Logf("data pipeline tools called: %v", toolCalls)
}

// ---------------------------------------------------------------------------
// Tool registrations
// ---------------------------------------------------------------------------

func registerPipelineTools(sess *core.Session) {
	tools := map[string]string{
		"write_file":  "write a file",
		"format_code": "format code",
		"run_test":    "run tests",
		"git_commit":  "commit changes",
	}
	for name, desc := range tools {
		n := name
		sess.Tools.Register(core.Tool{
			Name:        n,
			Description: desc,
			Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("[%s] done", n)}}}, nil
			},
		})
	}
}

func registerGitTools(sess *core.Session) {
	tools := map[string]string{
		"git_diff":   "show git diff",
		"git_commit": "commit changes",
		"git_log":    "show git log",
	}
	for name, desc := range tools {
		n := name
		sess.Tools.Register(core.Tool{
			Name:        n,
			Description: desc,
			Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("[%s] done", n)}}}, nil
			},
		})
	}
}

func registerWebTools(sess *core.Session) {
	tools := map[string]string{
		"web_search": "search the web",
		"web_fetch":  "fetch a URL",
	}
	for name, desc := range tools {
		n := name
		sess.Tools.Register(core.Tool{
			Name:        n,
			Description: desc,
			Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("[%s] done", n)}}}, nil
			},
		})
	}
}

func registerDataTools(sess *core.Session) {
	tools := map[string]string{
		"json_query": "query JSON data",
		"csv_query":  "query CSV data",
	}
	for name, desc := range tools {
		n := name
		sess.Tools.Register(core.Tool{
			Name:        n,
			Description: desc,
			Schema:      testSchema{raw: json.RawMessage(`{"type":"object"}`)},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("[%s] done", n)}}}, nil
			},
		})
	}
}
