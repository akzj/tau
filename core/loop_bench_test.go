package core_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// testSchema is a minimal core.ToolSchema for benchmarks.
type benchSchema struct {
	raw json.RawMessage
}

func (s benchSchema) Marshal() (json.RawMessage, error)     { return s.raw, nil }
func (s benchSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

func BenchmarkLoopPromptTurn(b *testing.B) {
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hello world"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, _ := core.NewSession(context.Background(), core.SessionOptions{Provider: prov})
	defer sess.Cancel()
	loop := core.NewLoop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		run, _ := loop.Prompt(context.Background(), sess, core.UserInput{Text: "hi"})
		for range run.Events() {
		}
		<-run.Done()
	}
}

func BenchmarkLoopPromptWithToolCall(b *testing.B) {
	b.StopTimer()
	loop := core.NewLoop()
	prov := faux.New()
	prov.QueueStream(faux.StreamResponse{
		Events: []core.ProviderEvent{
			{Type: core.ProvMessageStart, MessageID: "m1"},
			{Type: core.ProvToolCallStart, MessageID: "m1", ToolCallID: "c1", ToolName: "echo"},
			{Type: core.ProvToolCallDelta, MessageID: "m1", ToolCallID: "c1", ToolArgsDelta: `{"msg":"x"}`},
			{Type: core.ProvToolCallEnd, MessageID: "m1", ToolCallID: "c1"},
			{Type: core.ProvMessageEnd, MessageID: "m1"},
		},
	})

	sess, _ := core.NewSession(context.Background(), core.SessionOptions{Provider: prov})
	sess.Tools.Register(core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      benchSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ok"}}}, nil
		},
	})
	defer sess.Cancel()

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		run, _ := loop.Prompt(context.Background(), sess, core.UserInput{Text: "echo x"})
		for range run.Events() {
		}
		<-run.Done()
	}
}
