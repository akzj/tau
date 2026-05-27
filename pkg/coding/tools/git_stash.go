package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// GitStashTool creates a git stash operations tool.
//
// Parameters:
//
//	action  (string, required) — push | pop | list | apply | drop
//	message (string, optional) — stash message (for push)
//
// Executes git stash commands in the workspace root.
func GitStashTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Stash action: push, pop, list, apply, drop"},
			"message": {"type": "string", "description": "Stash message (for push action)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "git_stash",
		Description: "Git stash operations: push, pop, list, apply, drop. Executes git stash commands.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action  string `json:"action"`
				Message string `json:"message"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Action == "" {
				return core.ToolResult{}, fmt.Errorf("action required (push/pop/list/apply/drop)")
			}

			var cmd *exec.Cmd
			switch args.Action {
			case "push":
				if args.Message != "" {
					cmd = exec.CommandContext(ctx, "git", "stash", "push", "-m", args.Message)
				} else {
					cmd = exec.CommandContext(ctx, "git", "stash", "push")
				}
			case "pop":
				cmd = exec.CommandContext(ctx, "git", "stash", "pop")
			case "list":
				cmd = exec.CommandContext(ctx, "git", "stash", "list")
			case "apply":
				cmd = exec.CommandContext(ctx, "git", "stash", "apply")
			case "drop":
				cmd = exec.CommandContext(ctx, "git", "stash", "drop")
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use push/pop/list/apply/drop)", args.Action)
			}

			cmd.Dir = WorkspaceRoot
			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			cmd = exec.CommandContext(timeoutCtx, cmd.Path, cmd.Args[1:]...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git stash %s failed:\n%s", args.Action, text)}},
					Details: map[string]any{"action": args.Action, "success": false},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("git stash %s: done (no output)", args.Action)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"action": args.Action, "success": true},
			}, nil
		},
	}
}
