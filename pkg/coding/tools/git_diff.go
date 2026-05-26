package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/akzj/tau/core"
)

// GitDiffTool creates a git diff tool.
func GitDiffTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path (default: all)"},
			"staged": {"type": "boolean", "description": "Show staged changes only (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "git_diff",
		Description: "Show git changes: unstaged by default, --staged for staged only.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				Staged bool   `json:"staged"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			// Check git availability
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			var cmdArgs []string
			cmdArgs = append(cmdArgs, "diff")
			if args.Staged {
				cmdArgs = append(cmdArgs, "--staged")
			}
			if args.Path != "" {
				cmdArgs = append(cmdArgs, "--", args.Path)
			}

			cmd := exec.CommandContext(ctx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			output := stdout.String()
			if err != nil {
				errStr := stderr.String()
				if errStr != "" {
					output += "\n" + errStr
				}
			}
			if output == "" {
				output = "(no changes)"
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"staged": args.Staged, "path": args.Path},
			}, nil
		},
	}
}
