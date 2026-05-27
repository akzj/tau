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

// RunBenchTool creates a Go benchmark runner tool.
func RunBenchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"benchmark": {"type": "string", "description": "Benchmark regex (default .)"},
			"timeout": {"type": "string", "description": "Timeout (default 120s)"}
		}
	}`)

	return core.Tool{
		Name:        "run_bench",
		Description: "Run Go benchmarks with benchmem.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Benchmark string `json:"benchmark"`
				Timeout   string `json:"timeout"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Benchmark == "" {
				args.Benchmark = "."
			}
			if args.Timeout == "" {
				args.Timeout = "120s"
			}

			dur, err := time.ParseDuration(args.Timeout)
			if err != nil {
				dur = 120 * time.Second
				args.Timeout = "120s"
			}

			cmdCtx, cancel := context.WithTimeout(ctx, dur)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "go", "test",
				"-bench="+args.Benchmark, "-benchmem", "-count=1",
				"-timeout", args.Timeout, "./...")
			cmd.Dir = WorkspaceRoot

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr := cmd.Run()

			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}
			if runErr != nil {
				output += fmt.Sprintf("\nError: %v", runErr)
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"benchmark": args.Benchmark},
			}, nil
		},
	}
}
