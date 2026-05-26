package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/sandbox"
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

			if strings.TrimSpace(args.Command) == "" {
				return core.ToolResult{}, fmt.Errorf("bash: empty command")
			}

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

		// Route through container sandbox if available
		if SandboxRunner != nil && SandboxRunner.Backend() != sandbox.None {
			result, runErr := SandboxRunner.Run(cmdCtx, args.Command, workDir, time.Duration(args.TimeoutSeconds)*time.Second)
			if runErr == nil {
				output := result.Stdout
				if result.Stderr != "" {
					output += "\n[stderr]\n" + result.Stderr
				}
				if result.ExitCode != 0 {
					output += fmt.Sprintf("\n[exit: %d]", result.ExitCode)
				} else {
					output += "\n[exit: 0]"
				}
				if len(output) > OutputCap {
					output = output[:OutputCap] + "\n... (truncated)"
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
				}, nil
			}
			// Fall through to direct exec if container fails
		}

			cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
			cmd.Dir = workDir
			cmd.Env = FilteredEnv()

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			runErr := cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n[stderr]\n" + stderr.String()
			}

			// TruncateOutput saves full output to temp file if > TruncateCap
			if truncated, _ := TruncateOutput(output); truncated != output {
				output = truncated
			}

			// Build status trailer
			var status string
			if runErr != nil {
				if cmdCtx.Err() == context.DeadlineExceeded {
					status = fmt.Sprintf("[timeout after %ds]", args.TimeoutSeconds)
				} else if exitErr, ok := runErr.(*exec.ExitError); ok {
					if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
						if ws.Signaled() {
							status = fmt.Sprintf("[signal: %s]", ws.Signal())
						} else {
							status = fmt.Sprintf("[exit: %d]", ws.ExitStatus())
						}
					} else {
						status = fmt.Sprintf("[exit: error — %v]", runErr)
					}
				} else {
					status = fmt.Sprintf("[exit: error — %v]", runErr)
				}
			} else {
				status = "[exit: 0]"
			}

			// Cap output, leaving room for status
			statusOverhead := len(status) + 1 // +1 for newline
			if len(output)+statusOverhead > OutputCap {
				cut := OutputCap - statusOverhead - 15 // "... (truncated)"
				if cut < 0 {
					cut = 0
				}
				output = output[:cut] + "\n... (truncated)"
			}
			output += "\n" + status

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
			}, nil
		},
	}
}
