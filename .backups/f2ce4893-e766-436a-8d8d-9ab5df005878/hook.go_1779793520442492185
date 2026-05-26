package core

import "context"

// Chain is a multi-handler chain where each handler can transform the value.
// Handlers execute in insertion order.
type Chain[T any] interface {
	Add(handler func(context.Context, T) (T, error))
	Run(ctx context.Context, init T) (T, error)
}

// LastWins is a hook set where at most one owner is allowed (Set),
// but multiple observers are allowed (Observe). Observers fire after the owner.
type LastWins[T any] interface {
	Set(handler func(context.Context, T) (*T, error))
	Observe(handler func(context.Context, T))
	Run(ctx context.Context, init T) (T, error)
}

// HookSet bundles all hooks for a Session. Each hook is a plug point
// where product code can intercept and transform framework behaviour.
type HookSet struct {
	BeforeProviderRequest LastWins[StreamRequest]
	TransformContext      Chain[[]Message]
	BeforeToolCall        Chain[ToolCallEvent]
	AfterToolCall         Chain[ToolResultEvent]
}

// ToolCallEvent is passed to BeforeToolCall hooks.
type ToolCallEvent struct {
	CallID   string
	ToolName string
	Args     any
}

// ToolResultEvent is passed to AfterToolCall hooks.
type ToolResultEvent struct {
	CallID string
	Result ToolResult
	Err    error
}
