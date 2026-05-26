//go:build !no_plugins

package core

import (
	"context"
	"encoding/json"
	"testing"
)

// pluginTestSchema is a minimal ToolSchema for plugin tests.
type pluginTestSchema struct {
	raw json.RawMessage
}

func (s pluginTestSchema) Marshal() (json.RawMessage, error)  { return s.raw, nil }
func (s pluginTestSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// testPlugin implements Plugin for testing.
type testPlugin struct{}

func (p *testPlugin) Name() string    { return "test-plugin" }
func (p *testPlugin) Version() string { return "0.1.0" }
func (p *testPlugin) Tools() []Tool {
	return []Tool{{
		Name:        "test-echo",
		Description: "Test echo tool from plugin",
		Schema:      pluginTestSchema{raw: json.RawMessage(`{"type":"object"}`)},
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(PartialResult)) (ToolResult, error) {
			return ToolResult{Content: []Content{{Type: "text", Text: "echo from plugin"}}}, nil
		},
	}}
}
func (p *testPlugin) Providers() []Provider { return nil }

func TestPluginRegistry(t *testing.T) {
	RegisterPlugin(&testPlugin{})
	tools := AllPluginTools()
	if len(tools) != 1 {
		t.Errorf("expected 1 tool, got %d", len(tools))
	}
	if tools[0].Name != "test-echo" {
		t.Errorf("expected 'test-echo', got %q", tools[0].Name)
	}
}

func TestPluginToolExecution(t *testing.T) {
	RegisterPlugin(&testPlugin{})
	tools := AllPluginTools()
	result, err := tools[0].Execute(context.Background(), "c1", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Content[0].Text != "echo from plugin" {
		t.Errorf("unexpected result: %q", result.Content[0].Text)
	}
}
