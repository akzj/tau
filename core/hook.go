package core

import (
	"context"
	"time"
)

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
	BeforeCompaction      LastWins[CompactionRequest] // G4 Path A: summarise before compaction
	BeforeAgentStart      LastWins[AgentStartRequest] // ADD: fired at turn start
	AfterToolResult       Chain[ToolResultWithError]  // ADD: fired after each tool execution
	BeforeSessionTree     LastWins[TreeEntry]         // ADD: fired before tree insertion
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

// CompactionRequest is passed to BeforeCompaction hooks when the transcript
// exceeds the configured token threshold. The handler produces a summary;
// core replaces messages before FirstKeptEntryID with the summary.
type CompactionRequest struct {
	Summary          string   // handler fills this with the summarization result
	FirstKeptEntryID Position // first message ID to keep after compaction
	TokensBefore     int      // estimated token count before compaction
}

// AgentStartRequest is passed to BeforeAgentStart hooks when a Loop run begins.
type AgentStartRequest struct {
	TurnID string
	Input  string // user input text (empty for Continue)
	Turn   int    // 1-based turn number
}

// ToolResultWithError is passed to AfterToolResult hooks after a tool executes.
type ToolResultWithError struct {
	CallID string
	Result ToolResult
	Err    error
}

// TreeEntry is passed to BeforeSessionTree hooks before insertion into the session tree.
type TreeEntry struct {
	ID        string
	ParentID  string
	Type      string    // "message", "compaction", "leaf", "branch_summary", "session_info"
	Timestamp time.Time
	Data      any // the actual entry data (Message, CompactionRequest, etc.)
}

// --- Chain implementation ---

type chainImpl[T any] struct {
	handlers []func(context.Context, T) (T, error)
}

// NewChain creates an empty Chain.
func NewChain[T any]() Chain[T] {
	return &chainImpl[T]{}
}

func (c *chainImpl[T]) Add(handler func(context.Context, T) (T, error)) {
	c.handlers = append(c.handlers, handler)
}

func (c *chainImpl[T]) Run(ctx context.Context, init T) (T, error) {
	val := init
	for _, h := range c.handlers {
		var err error
		val, err = h(ctx, val)
		if err != nil {
			return val, err
		}
	}
	return val, nil
}

// --- LastWins implementation ---

type lastWinsImpl[T any] struct {
	owner     func(context.Context, T) (*T, error)
	observers []func(context.Context, T)
	setCalled bool
}

// NewLastWins creates an empty LastWins hook.
func NewLastWins[T any]() LastWins[T] {
	return &lastWinsImpl[T]{}
}

func (lw *lastWinsImpl[T]) Set(handler func(context.Context, T) (*T, error)) {
	if lw.setCalled {
		panic("LastWins.Set called twice — duplicate owner handler (P2 panic-fast)")
	}
	lw.owner = handler
	lw.setCalled = true
}

func (lw *lastWinsImpl[T]) Observe(handler func(context.Context, T)) {
	lw.observers = append(lw.observers, handler)
}

func (lw *lastWinsImpl[T]) Run(ctx context.Context, init T) (T, error) {
	// Observers run first (read-only)
	for _, obs := range lw.observers {
		obs(ctx, init)
	}
	// Owner runs last, can override value
	if lw.owner != nil {
		result, err := lw.owner(ctx, init)
		if err != nil {
			return init, err
		}
		if result != nil {
			return *result, nil
		}
	}
	return init, nil
}
