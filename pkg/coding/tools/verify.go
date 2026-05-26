package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
	"github.com/akzj/tau/pkg/sandbox"
)

// VerifyTool creates a code verification tool.
// Runs a command, checks output against expected regex pattern.
// Integrates with Docker sandbox when available; falls back to local exec.
func VerifyTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "Shell command to run"},
			"expected_output": {"type": "string", "description": "Regex pattern the output must match"},
			"timeout_seconds": {"type": "integer", "description": "Command timeout (default 30, max 120)"},
			"work_dir": {"type": "string", "description": "Working directory (default: workspace root)"}
		},
		"required": ["command"]
	}`)

	return core.Tool{
		Name:        "verify",
		Description: "Run a command and verify its output against an expected regex pattern. Used for self-correction: write code→verify→fail→fix→re-verify.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Command        string `json:"command"`
				ExpectedOutput string `json:"expected_output"`
				TimeoutSeconds int    `json:"timeout_seconds"`
				WorkDir        string `json:"work_dir"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.Command == "" {
				return core.ToolResult{}, fmt.Errorf("command required")
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

			// Try Docker sandbox first, fall back to local exec
			var stdout, stderr bytes.Buffer
			exitCode := -1
			sandboxed := false

			if SandboxRunner != nil && SandboxRunner.Backend() != sandbox.None {
				result, err := SandboxRunner.Run(ctx, args.Command, workDir, time.Duration(args.TimeoutSeconds)*time.Second)
				if err == nil {
					stdout.WriteString(result.Stdout)
					stderr.WriteString(result.Stderr)
					exitCode = result.ExitCode
					sandboxed = true
				}
			}

			if !sandboxed {
				cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(args.TimeoutSeconds)*time.Second)
				defer cancel()
				cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
				cmd.Dir = workDir
				cmd.Env = FilteredEnv()
				cmd.Stdout = &stdout
				cmd.Stderr = &stderr
				err := cmd.Run()
				if err != nil {
					if cmdCtx.Err() == context.DeadlineExceeded {
						return core.ToolResult{
							Content: []core.Content{{Type: "text", Text: fmt.Sprintf("FAIL: timeout after %ds\nstdout:\n%s\nstderr:\n%s",
								args.TimeoutSeconds, stdout.String(), stderr.String())}},
							Details: map[string]any{"passed": false, "exit_code": -1, "sandboxed": sandboxed},
						}, nil
					}
					if exitErr, ok := err.(*exec.ExitError); ok {
						exitCode = exitErr.ExitCode()
					}
				} else {
					exitCode = 0
				}
			}

			output := stdout.String()
			passed := true
			var reason string

			if args.ExpectedOutput != "" {
				re, err := regexp.Compile(args.ExpectedOutput)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
				}
				if !re.MatchString(output) {
					passed = false
					reason = fmt.Sprintf("output did not match expected pattern: %s", args.ExpectedOutput)
				}
			} else if exitCode != 0 {
				passed = false
				reason = fmt.Sprintf("command exited with code %d", exitCode)
			}

			var result strings.Builder
			if passed {
				result.WriteString("✅ PASS\n")
			} else {
				result.WriteString("❌ FAIL")
				if reason != "" {
					result.WriteString(" — " + reason)
				}
				result.WriteString("\n")
			}
			result.WriteString(fmt.Sprintf("exit: %d\n", exitCode))
			if output != "" {
				if len(output) > 4000 {
					output = output[:4000] + "\n... (truncated)"
				}
				result.WriteString(fmt.Sprintf("stdout:\n%s\n", output))
			}
			if stderr.Len() > 0 {
				result.WriteString(fmt.Sprintf("stderr:\n%s\n", stderr.String()))
			}
			if sandboxed {
				result.WriteString("[sandbox: docker]\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: result.String()}},
				Details: map[string]any{"passed": passed, "exit_code": exitCode, "expected": args.ExpectedOutput, "sandboxed": sandboxed},
			}, nil
		},
	}
}
