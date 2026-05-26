package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/akzj/tau/core"
)

// DepsTool creates a module dependency listing tool.
func DepsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path to analyze (default: root module)"},
			"format": {"type": "string", "description": "Output format: tree or list (default list)"}
		}
	}`)

	return core.Tool{
		Name:        "deps",
		Description: "Show module dependency tree using go mod graph or go list.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				Format string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Format == "" {
				args.Format = "list"
			}

			cmdArgs := []string{"list", "-m", "-json", "all"}
			cmd := exec.CommandContext(ctx, "go", cmdArgs...)
			cmd.Dir = WorkspaceRoot
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			if output == "" {
				output = "(no dependencies or go not found)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"path": args.Path},
			}, nil
		},
	}
}
