package tools

import (
	"context"

	"github.com/akzj/tau/core"
)

// toolThreePhase is a reusable ThreePhaseTool adapter for tools that
// want to separate validation (Prepare) from execution (Execute).
type toolThreePhase struct {
	prepare  func(ctx context.Context, callID string, params any) (core.PreparedTool, error)
	execute  func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error)
	finalize func(ctx context.Context, prepared core.PreparedTool, result core.ToolResult) error
}

func (t *toolThreePhase) Prepare(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
	return t.prepare(ctx, callID, params)
}

func (t *toolThreePhase) Execute(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
	return t.execute(ctx, prepared, onUpdate)
}

func (t *toolThreePhase) Finalize(ctx context.Context, prepared core.PreparedTool, result core.ToolResult) error {
	if t.finalize != nil {
		return t.finalize(ctx, prepared, result)
	}
	return nil
}
