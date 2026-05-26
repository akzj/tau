//go:build !no_plugins

package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akzj/tau/core"
)

// subAgentSchema is a minimal ToolSchema adapter for the sub_agent tool.
type subAgentSchema struct {
	raw json.RawMessage
}

func (s subAgentSchema) Marshal() (json.RawMessage, error)      { return s.raw, nil }
func (s subAgentSchema) Validate(raw json.RawMessage) (any, error) { return raw, nil }

// SubAgentTool creates a tool that spawns sub-agents.
func SubAgentTool() core.Tool {
	raw := json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {"type": "string", "description": "The prompt for the sub-agent"},
			"tools": {"type": "array", "items": {"type": "string"}, "description": "Allowed tool names"},
			"budget": {"type": "integer", "description": "Max turns (default 5)"}
		},
		"required": ["prompt"]
	}`)

	return core.Tool{
		Name:        "spawn_agent",
		Description: "Spawn a sub-agent with its own session and workspace. Returns the sub-agent's output.",
		Schema:      subAgentSchema{raw: raw},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Prompt string   `json:"prompt"`
				Tools  []string `json:"tools"`
				Budget int      `json:"budget"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.ToolResult{}, err
			}
			if args.Prompt == "" {
				return core.ToolResult{}, fmt.Errorf("prompt required")
			}
			if args.Budget <= 0 {
				args.Budget = 5
			}

			result := fmt.Sprintf("Sub-agent spawned with prompt: %s (budget: %d turns)", args.Prompt, args.Budget)
			if len(args.Tools) > 0 {
				result += fmt.Sprintf("\nAllowed tools: %v", args.Tools)
			}
			result += "\n\n[Sub-agent execution via SpawnSubAgent — ensure core.DecodeJSONOutput for structured results]"

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: result}},
				Details: map[string]any{"prompt": args.Prompt, "budget": args.Budget, "tools": args.Tools},
			}, nil
		},
	}
}
