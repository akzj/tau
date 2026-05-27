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

// GitCherryPickTool creates a git cherry-pick tool.
//
// Parameters:
//
//	commit (string, required) — commit hash to cherry-pick
//	no_commit (bool, optional) — stage changes without committing
func GitCherryPickTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"commit": {"type": "string", "description": "Commit hash to cherry-pick"},
			"no_commit": {"type": "boolean", "description": "Stage changes without committing"}
		},
		"required": ["commit"]
	}`)

	return core.Tool{
		Name:        "git_cherry_pick",
		Description: "Cherry-pick a git commit by hash. Supports --no-commit for staging only.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Commit   string `json:"commit"`
				NoCommit bool   `json:"no_commit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Commit == "" {
				return core.ToolResult{}, fmt.Errorf("commit hash required")
			}

			cmdArgs := []string{"cherry-pick"}
			if args.NoCommit {
				cmdArgs = append(cmdArgs, "--no-commit")
			}
			cmdArgs = append(cmdArgs, args.Commit)

			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(timeoutCtx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git cherry-pick failed:\n%s", text)}},
					Details: map[string]any{"commit": args.Commit, "success": false},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("git cherry-pick %s: done", args.Commit)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"commit": args.Commit, "success": true},
			}, nil
		},
	}
}
