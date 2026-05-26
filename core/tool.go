package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// ExecutionMode describes how tools are dispatched.
type ExecutionMode int

const (
	ModeSequential ExecutionMode = iota
	ModeParallel
)

// Content is a piece of tool result content.
type Content struct {
	Type string // "text" | "image"
	Text string
	Data []byte // for images
}

// PartialResult represents a partial tool execution result.
type PartialResult struct {
	ContentDelta string
	Done         bool
}

// ToolResult is the canonical tool output.
type ToolResult struct {
	Content   []Content
	Details   any
	Terminate bool // hint: all tools in batch must set true for actual stop
}

// ToolExecuteFn is the synchronous callback that implements a tool's behaviour.
// onUpdate may be called zero or more times with partial results.
type ToolExecuteFn func(
	ctx context.Context,
	callID string,
	params any,
	onUpdate func(PartialResult),
) (ToolResult, error)

// PreparedTool holds the result of a tool's prepare phase.
type PreparedTool struct {
	CallID   string
	ToolName string
	Params   any // validated/transformed params for Execute
	State    any // opaque state passed to Finalize
}

// ThreePhaseTool is an optional interface for tools that want
// separate prepare / execute / finalize phases.
// Loop detects this interface and uses the three-phase path;
// otherwise falls back to single-stage Tool.Execute.
type ThreePhaseTool interface {
	// Prepare validates inputs and returns a PreparedTool.
	// Called synchronously in the event loop.
	Prepare(ctx context.Context, callID string, params any) (PreparedTool, error)

	// Execute runs the tool logic. Called in a goroutine for parallel execution.
	Execute(ctx context.Context, prepared PreparedTool, onUpdate func(PartialResult)) (ToolResult, error)

	// Finalize cleans up after execution (e.g., close temp files, log).
	// Called after Execute completes, regardless of error.
	Finalize(ctx context.Context, prepared PreparedTool, result ToolResult) error
}

// ToolSchema is the three-in-one contract: typed Go struct, LLM JSON Schema, runtime validator.
// Implementations live in toolspec/ (codegen-generated).
type ToolSchema interface {
	Marshal() (json.RawMessage, error)        // → LLM-facing JSON Schema
	Validate(raw json.RawMessage) (any, error) // ← LLM-returned JSON → validated+decoded
}

// Tool is the canonical tool abstraction.
type Tool struct {
	Name        string
	Description string
	Schema      ToolSchema // JSON Schema for LLM (implements Marshal/Validate)
	Execute     ToolExecuteFn
	PrepareArgs func(raw json.RawMessage) (any, error) // optional pre-validate
	Mode        ExecutionMode
	ThreePhase  ThreePhaseTool // optional three-phase implementation
}

// ToolRegistry is Session-scoped.
type ToolRegistry struct {
	tools  map[string]Tool
	active map[string]bool
}

// NewToolRegistry creates an empty ToolRegistry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools:  make(map[string]Tool),
		active: make(map[string]bool),
	}
}

// Register adds a Tool to the registry. Tools are active by default.
func (r *ToolRegistry) Register(t Tool) error {
	if t.Name == "" {
		return fmt.Errorf("tool name must not be empty")
	}
	if _, exists := r.tools[t.Name]; exists {
		return fmt.Errorf("tool %q already registered", t.Name)
	}
	r.tools[t.Name] = t
	r.active[t.Name] = true
	return nil
}

// Get returns a Tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// Active returns all currently active Tools.
func (r *ToolRegistry) Active() []Tool {
	var result []Tool
	for name, t := range r.tools {
		if r.active[name] {
			result = append(result, t)
		}
	}
	return result
}

// SetActive replaces the active set with exactly the named tools.
func (r *ToolRegistry) SetActive(names []string) {
	r.active = make(map[string]bool)
	for _, n := range names {
		if _, ok := r.tools[n]; ok {
			r.active[n] = true
		}
	}
}
