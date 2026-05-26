package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/akzj/tau/core"
)

// BashTool creates a bash-execution tool.
func BashTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Bash command to execute"},
			"work_dir": {"type": "string", "description": "Working directory relative to workspace root (optional)"},
			"timeout_seconds": {"type": "integer", "description": "Command timeout in seconds (max 120, default 30)"}
		},
		"required": ["command"]
	}`)

	return core.Tool{
		Name:        "bash",
		Description: "Execute a bash command in the workspace.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Command        string `json:"command"`
				WorkDir        string `json:"work_dir"`
				TimeoutSeconds int    `json:"timeout_seconds"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.TimeoutSeconds <= 0 {
				args.TimeoutSeconds = 30
			}
			if args.TimeoutSeconds > 120 {
				args.TimeoutSeconds = 120
			}

			workDir := WorkspaceRoot
			if args.WorkDir != "" {
				var err error
				workDir, err = ResolvePath(args.WorkDir)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(args.TimeoutSeconds)*time.Second)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
			cmd.Dir = workDir
			cmd.Env = FilteredEnv()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			err := cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n[stderr]\n" + stderr.String()
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			if err != nil {
				if cmdCtx.Err() == context.DeadlineExceeded {
					return core.ToolResult{
						Content: []core.Content{{Type: "text", Text: output + "\n[TIMEOUT]"}},
					}, nil
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output + fmt.Sprintf("\n[EXIT: %v]", err)}},
				}, nil
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
			}, nil
		},
	}
}
