package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

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
		Description: "Show git changes: unstaged by default, --staged for staged only. Includes --stat summary.",
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

			// Diff
			var diffArgs []string
			diffArgs = append(diffArgs, "diff")
			if args.Staged {
				diffArgs = append(diffArgs, "--staged")
			}
			if args.Path != "" {
				diffArgs = append(diffArgs, "--", args.Path)
			}

			cmd := exec.CommandContext(ctx, "git", diffArgs...)
			cmd.Dir = WorkspaceRoot
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}

			// --stat summary
			var statArgs []string
			statArgs = append(statArgs, "diff", "--stat")
			if args.Staged {
				statArgs = append(statArgs, "--staged")
			}
			if args.Path != "" {
				statArgs = append(statArgs, "--", args.Path)
			}
			statCmd := exec.CommandContext(ctx, "git", statArgs...)
			statCmd.Dir = WorkspaceRoot
			statOut, _ := statCmd.Output()
			summary := strings.TrimSpace(string(statOut))
			if summary != "" {
				output += "\n\n--- Stat ---\n" + summary
			}

			if output == "" {
				output = "(no changes)"
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"staged":  args.Staged,
					"path":    args.Path,
					"summary": summary,
				},
			}, nil
		},
	}
}
