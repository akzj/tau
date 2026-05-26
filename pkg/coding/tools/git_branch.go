package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/akzj/tau/core"
)

// GitBranchTool creates a git branch management tool.
func GitBranchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["list", "create"], "description": "list branches or create a new one"},
			"name": {"type": "string", "description": "Branch name (required for create)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "git_branch",
		Description: "List or create git branches. Create requires confirmation (IsDangerous).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Name   string `json:"name"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			switch args.Action {
			case "list":
				cmd := exec.CommandContext(ctx, "git", "branch", "--list")
				cmd.Dir = WorkspaceRoot
				out, _ := cmd.Output()
				output := string(out)
				if output == "" {
					output = "(no branches)"
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"action": "list"},
				}, nil
			case "create":
				if args.Name == "" {
					return core.ToolResult{}, fmt.Errorf("branch name required")
				}
				cmd := exec.CommandContext(ctx, "git", "branch", args.Name)
				cmd.Dir = WorkspaceRoot
				out, err := cmd.CombinedOutput()
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("git branch: %s", string(out))
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("✅ Created branch: %s", args.Name)}},
					Details: map[string]any{"action": "create", "name": args.Name},
				}, nil
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s", args.Action)
			}
		},
	}
}
