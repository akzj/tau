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

// GitRevertTool creates a git revert tool.
//
// Parameters:
//
//	commit  (string, required) — commit hash to revert
//	message (string, optional) — custom revert message
//	no_edit (bool, optional) — skip commit message editor
func GitRevertTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"commit": {"type": "string", "description": "Commit hash to revert"},
			"message": {"type": "string", "description": "Custom revert commit message"},
			"no_edit": {"type": "boolean", "description": "Skip commit message editor (default: true)"}
		},
		"required": ["commit"]
	}`)

	return core.Tool{
		Name:        "git_revert",
		Description: "Revert a git commit by hash. Supports custom message and --no-edit flag.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Commit  string `json:"commit"`
				Message string `json:"message"`
				NoEdit  bool   `json:"no_edit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Commit == "" {
				return core.ToolResult{}, fmt.Errorf("commit hash required")
			}

			cmdArgs := []string{"revert", args.Commit}
			if args.NoEdit {
				cmdArgs = append(cmdArgs, "--no-edit")
			}
			if args.Message != "" {
				cmdArgs = append(cmdArgs, "-m", args.Message)
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(timeoutCtx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git revert failed:\n%s", text)}},
					Details: map[string]any{"commit": args.Commit, "success": false},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("git revert %s: done", args.Commit)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"commit": args.Commit, "success": true},
			}, nil
		},
	}
}
