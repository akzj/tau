package core_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/akzj/tau/core"
)

func BenchmarkToolExecution(b *testing.B) {
	tool := core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      benchSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ok"}}}, nil
		},
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool.Execute(ctx, "c1", map[string]any{"msg": "hello"}, nil)
	}
}

func BenchmarkToolThreePhase(b *testing.B) {
	tool := core.Tool{
		Name:        "echo",
		Description: "echo",
		Schema:      benchSchema{raw: json.RawMessage(`{"type":"object"}`)},
		ThreePhase:  &benchThreePhase{},
	}

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		raw, _ := tool.ThreePhase.PrepareArgsRaw(json.RawMessage(`{"msg":"hello"}`))
		prepared, _ := tool.ThreePhase.Prepare(ctx, "c1", raw)
		_, _ = tool.ThreePhase.Execute(ctx, prepared, nil)
		_ = tool.ThreePhase.Finalize(ctx, prepared, core.ToolResult{})
	}
}

type benchThreePhase struct{}

func (t *benchThreePhase) PrepareArgsRaw(rawArgs json.RawMessage) (json.RawMessage, error) {
	return rawArgs, nil
}
func (t *benchThreePhase) Prepare(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
	return core.PreparedTool{CallID: callID, ToolName: "echo", Params: params}, nil
}
func (t *benchThreePhase) Execute(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
	return core.ToolResult{Content: []core.Content{{Type: "text", Text: "ok"}}}, nil
}
func (t *benchThreePhase) Finalize(ctx context.Context, prepared core.PreparedTool, result core.ToolResult) error {
	return nil
}
