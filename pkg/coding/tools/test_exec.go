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

// RunTestTool creates a focused go test runner tool.
func RunTestTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Test path (package or ./...)"},
			"timeout": {"type": "string", "description": "Test timeout (default 60s)"},
			"verbose": {"type": "boolean", "description": "Verbose output"}
		}
	}`)

	return core.Tool{
		Name:        "run_test",
		Description: "Run go test on specified packages.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string `json:"path"`
				Timeout string `json:"timeout"`
				Verbose bool   `json:"verbose"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = "./..."
			}
			if args.Timeout == "" {
				args.Timeout = "60s"
			}

			// Parse timeout to avoid negative durations
			dur, err := time.ParseDuration(args.Timeout)
			if err != nil {
				dur = 60 * time.Second
				args.Timeout = "60s"
			}

			cmdCtx, cancel := context.WithTimeout(ctx, dur)
			defer cancel()

			cmdArgs := []string{"test", "-count=1", "-timeout", args.Timeout}
			if args.Verbose {
				cmdArgs = append(cmdArgs, "-v")
			}
			cmdArgs = append(cmdArgs, args.Path)
			cmd := exec.CommandContext(cmdCtx, "go", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n[stderr]\n" + stderr.String()
			}
			passed := runErr == nil
			if cmdCtx.Err() == context.DeadlineExceeded {
				output += "\n[timeout]"
			} else if runErr != nil {
				output += fmt.Sprintf("\n[exit: %v]", runErr)
			} else {
				output += "\n[pass]"
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"passed": passed, "path": args.Path},
			}, nil
		},
	}
}
