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

// Tool is the canonical tool abstraction.
type Tool struct {
	Name        string
	Description string
	Schema      json.RawMessage // JSON Schema for LLM
	Execute     ToolExecuteFn
	Mode        ExecutionMode
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
