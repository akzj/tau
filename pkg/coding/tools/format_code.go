package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/akzj/tau/core"
)

// FormatCodeTool creates a code formatter tool using gofmt/goimports.
func FormatCodeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path (default: workspace root)"},
			"write": {"type": "boolean", "description": "Write changes (default false, dry-run shows diff)"}
		}
	}`)

	return core.Tool{
		Name:        "format_code",
		Description: "Format Go code with gofmt or goimports. Dry-run shows diff by default.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path  string `json:"path"`
				Write bool   `json:"write"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = WorkspaceRoot
			}

			// Try goimports first, fallback to gofmt
			tool := "goimports"
			if _, err := exec.LookPath("goimports"); err != nil {
				tool = "gofmt"
			}
			if _, err := exec.LookPath("gofmt"); err != nil {
				return core.ToolResult{}, fmt.Errorf("gofmt not available")
			}

			cmdArgs := []string{"-l", "-d", args.Path}
			if args.Write {
				cmdArgs = append(cmdArgs, "-w")
			}
			cmd := exec.CommandContext(ctx, tool, cmdArgs...)
			cmd.Dir = WorkspaceRoot

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}
			if output == "" {
				if args.Write {
					output = fmt.Sprintf("Formatted %s", args.Path)
				} else {
					output = "No formatting issues found."
				}
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"path": args.Path, "write": args.Write},
			}, nil
		},
	}
}
