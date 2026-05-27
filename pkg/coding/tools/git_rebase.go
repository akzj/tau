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

// GitRebaseTool creates a git rebase tool.
//
// Parameters:
//
//	branch   (string, required) — target branch to rebase onto
//	interactive (bool, optional) — use interactive rebase
//	abort    (bool, optional) — abort in-progress rebase
//	continue_ (bool, optional) — continue in-progress rebase
func GitRebaseTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"branch": {"type": "string", "description": "Target branch to rebase onto"},
			"interactive": {"type": "boolean", "description": "Use interactive rebase"},
			"abort": {"type": "boolean", "description": "Abort in-progress rebase"},
			"continue": {"type": "boolean", "description": "Continue in-progress rebase after resolving conflicts"}
		},
		"required": []
	}`)

	return core.Tool{
		Name:        "git_rebase",
		Description: "Git rebase operations. Rebase current branch onto target, or abort/continue in-progress rebase.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Branch      string `json:"branch"`
				Interactive bool   `json:"interactive"`
				Abort       bool   `json:"abort"`
				Continue    bool   `json:"continue"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			var cmdArgs []string

			if args.Abort {
				cmdArgs = []string{"rebase", "--abort"}
			} else if args.Continue {
				cmdArgs = []string{"rebase", "--continue"}
			} else if args.Branch != "" {
				cmdArgs = []string{"rebase"}
				if args.Interactive {
					cmdArgs = append(cmdArgs, "-i")
				}
				cmdArgs = append(cmdArgs, args.Branch)
			} else {
				return core.ToolResult{}, fmt.Errorf("either branch, abort, or continue required")
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			defer cancel()
			cmd := exec.CommandContext(timeoutCtx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git rebase failed:\n%s", text)}},
					Details: map[string]any{"branch": args.Branch, "success": false},
				}, err
			}

			if text == "" {
				text = "git rebase: done (no output)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"branch": args.Branch, "success": true},
			}, nil
		},
	}
}
