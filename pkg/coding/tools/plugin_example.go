//go:build !no_plugins

package tools

import (
	"context"
	"encoding/json"

	"github.com/akzj/tau/core"
)

// ExamplePlugin implements core.Plugin as a reference implementation.
type ExamplePlugin struct{}

func (p *ExamplePlugin) Name() string    { return "tau-example" }
func (p *ExamplePlugin) Version() string { return "0.1.0" }

func (p *ExamplePlugin) Tools() []core.Tool {
	return []core.Tool{{
		Name:        "plugin-echo",
		Description: "Echo tool provided by the tau-example plugin",
		Schema:      rawToolSchema{raw: json.RawMessage(`{"type":"object","properties":{"msg":{"type":"string"}}}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			return core.ToolResult{Content: []core.Content{{Type: "text", Text: "plugin echo: hello"}}}, nil
		},
	}}
}

func (p *ExamplePlugin) Providers() []core.Provider { return nil }

// rawToolSchema wraps a json.RawMessage as core.ToolSchema.
type rawToolSchema struct {
	raw json.RawMessage
}

func (s rawToolSchema) Marshal() (json.RawMessage, error)      { return s.raw, nil }
func (s rawToolSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// RegisterExample registers the example plugin.
func RegisterExample() {
	core.RegisterPlugin(&ExamplePlugin{})
}
