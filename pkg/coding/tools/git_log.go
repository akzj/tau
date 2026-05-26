package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/akzj/tau/core"
)

// GitLogTool creates a git log tool.
func GitLogTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"count": {"type": "integer", "description": "Number of commits (default 20, max 100)"},
			"path": {"type": "string", "description": "File or directory path (optional)"},
			"oneline": {"type": "boolean", "description": "One-line format (default true)"}
		}
	}`)

	return core.Tool{
		Name:        "git_log",
		Description: "Show commit history. Returns hash, author, date, message.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Count   int    `json:"count"`
				Path    string `json:"path"`
				Oneline bool   `json:"oneline"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Count <= 0 {
				args.Count = 20
			}
			if args.Count > 100 {
				args.Count = 100
			}
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			cmdArgs := []string{"log", fmt.Sprintf("-%d", args.Count)}
			if args.Oneline {
				cmdArgs = append(cmdArgs, "--oneline")
			}
			cmdArgs = append(cmdArgs, "--format=%h %an %ad %s", "--date=short")
			if args.Path != "" {
				cmdArgs = append(cmdArgs, "--", args.Path)
			}

			cmd := exec.CommandContext(ctx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot
			out, err := cmd.Output()
			output := string(out)
			if err != nil {
				output += fmt.Sprintf("\n[git error: %v]", err)
			}
			if output == "" {
				output = "(no commits)"
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"count": args.Count, "path": args.Path},
			}, nil
		},
	}
}
