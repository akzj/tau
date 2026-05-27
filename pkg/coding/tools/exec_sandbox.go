package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akzj/tau/core"
)

// ExecSandboxTool creates an exec_sandbox tool that runs commands
// inside a sandboxed environment (Docker or local fallback).
//
// Parameters:
//
//	command  (string, required) — shell command to execute
//	work_dir (string, optional) — working directory (default: workspace root)
//	timeout  (integer, optional) — timeout in seconds (default: 300, max: 600)
//
// Returns exit code, stdout, stderr, duration, and sandbox status.
func ExecSandboxTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to execute in sandbox"},
			"work_dir": {"type": "string", "description": "Working directory (default: workspace root)"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default: 300, max: 600)"}
		},
		"required": ["command"]
	}`)

	return core.Tool{
		Name:        "exec_sandbox",
		Description: "Execute a command in a sandboxed environment (Docker with network isolation, CPU/memory limits, read-only rootfs; falls back to local execution with a warning if Docker is unavailable).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Command string `json:"command"`
				WorkDir string `json:"work_dir"`
				Timeout int    `json:"timeout"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.Command == "" {
				return core.ToolResult{}, fmt.Errorf("exec_sandbox: command required")
			}
			if args.WorkDir == "" {
				args.WorkDir = WorkspaceRoot
			}

			result, err := SandboxRunner.Run(ctx, args.Command, args.WorkDir)
			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("❌ exec_sandbox failed: %v", err)}},
					Details: map[string]any{"exit_code": -1, "error": err.Error()},
				}, nil
			}

			status := "✅"
			if result.ExitCode != 0 {
				status = "❌"
			}

			sandboxTag := "local"
			if result.Sandboxed {
				sandboxTag = "docker"
			}

			var output string
			output += fmt.Sprintf("%s exit: %d | duration: %s | sandbox: %s\n\n",
				status, result.ExitCode, result.Duration, sandboxTag)
			if result.Stdout != "" {
				output += fmt.Sprintf("stdout:\n%s\n", result.Stdout)
			}
			if result.Stderr != "" {
				output += fmt.Sprintf("stderr:\n%s\n", result.Stderr)
			}
			if result.Error != "" {
				output += fmt.Sprintf("error:\n%s\n", result.Error)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"exit_code": result.ExitCode,
					"sandboxed": result.Sandboxed,
					"backend":   SandboxRunner.Backend(),
					"duration":  result.Duration,
				},
			}, nil
		},
	}
}