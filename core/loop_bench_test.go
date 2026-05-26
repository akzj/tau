package core_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/testing/faux"
)

// benchSchema is a minimal core.ToolSchema for benchmarks.
type benchSchema struct {
	raw json.RawMessage
}

func (s benchSchema) Marshal() (json.RawMessage, error)     { return s.raw, nil }
func (s benchSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

func BenchmarkLoopPromptTurn(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		prov := faux.New()
		prov.QueueStream(faux.StreamResponse{
			Events: []core.ProviderEvent{
				{Type: core.ProvMessageStart, MessageID: "m1"},
				{Type: core.ProvContentDelta, MessageID: "m1", ContentDelta: "hello world"},
				{Type: core.ProvMessageEnd, MessageID: "m1"},
			},
		})
		sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
		loop := core.NewLoop()
		b.StartTimer()

		run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "hi"})
		for range run.Events() {
		}
		<-run.Done()
		sess.Cancel()
	}
}

func BenchmarkLoopPromptWithToolCall(b *testing.B) {
	ctx := context.Background()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
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
		sess, _ := core.NewSession(ctx, core.SessionOptions{Provider: prov})
		sess.Tools.Register(core.Tool{
			Name:        "echo",
			Description: "echo",
			Schema:      benchSchema{raw: json.RawMessage(`{"type":"object"}`)},
			Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ok"}}}, nil
			},
		})
		loop := core.NewLoop()
		b.StartTimer()

		run, _ := loop.Prompt(ctx, sess, core.UserInput{Text: "echo x"})
		for range run.Events() {
		}
		<-run.Done()
		sess.Cancel()
	}
}
