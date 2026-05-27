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

// GitTagTool creates a git tag operations tool.
//
// Parameters:
//
//	action  (string, required) — list | create | delete | push
//	name    (string, required for create/delete/push) — tag name
//	message (string, optional) — tag message (for create -a)
//	remote  (string, optional) — remote name for push (default "origin")
func GitTagTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Tag action: list, create, delete, push"},
			"name": {"type": "string", "description": "Tag name"},
			"message": {"type": "string", "description": "Tag message (for annotated tags)"},
			"remote": {"type": "string", "description": "Remote name for push (default: origin)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "git_tag",
		Description: "Git tag operations: list, create, delete, push tags.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action  string `json:"action"`
				Name    string `json:"name"`
				Message string `json:"message"`
				Remote  string `json:"remote"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Action == "" {
				return core.ToolResult{}, fmt.Errorf("action required (list/create/delete/push)")
			}
			if args.Remote == "" {
				args.Remote = "origin"
			}

			var cmd *exec.Cmd
			switch args.Action {
			case "list":
				cmd = exec.CommandContext(ctx, "git", "tag", "--list")
			case "create":
				if args.Name == "" {
					return core.ToolResult{}, fmt.Errorf("name required for create")
				}
				if args.Message != "" {
					cmd = exec.CommandContext(ctx, "git", "tag", "-a", args.Name, "-m", args.Message)
				} else {
					cmd = exec.CommandContext(ctx, "git", "tag", args.Name)
				}
			case "delete":
				if args.Name == "" {
					return core.ToolResult{}, fmt.Errorf("name required for delete")
				}
				cmd = exec.CommandContext(ctx, "git", "tag", "-d", args.Name)
			case "push":
				if args.Name == "" {
					return core.ToolResult{}, fmt.Errorf("name required for push")
				}
				cmd = exec.CommandContext(ctx, "git", "push", args.Remote, args.Name)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use list/create/delete/push)", args.Action)
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
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git tag %s failed:\n%s", args.Action, text)}},
					Details: map[string]any{"action": args.Action, "success": false},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("git tag %s: done (no output)", args.Action)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"action": args.Action, "success": true},
			}, nil
		},
	}
}
