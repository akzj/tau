package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/akzj/tau/core"
)

// LintCodeTool creates a go vet-based linting tool.
func LintCodeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path (default ./...)"}
		}
	}`)

	return core.Tool{
		Name:        "lint_code",
		Description: "Lint Go code with go vet. Returns issues found.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path string `json:"path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = "./..."
			}

			cmd := exec.CommandContext(ctx, "go", "vet", args.Path)
			cmd.Dir = WorkspaceRoot

			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()

			var result strings.Builder
			stdoutStr := stdout.String()
			stderrStr := stderr.String()

			if stdoutStr != "" || stderrStr != "" {
				result.WriteString("=== go vet issues ===\n")
				if stdoutStr != "" {
					result.WriteString(stdoutStr)
				}
				if stderrStr != "" {
					result.WriteString(stderrStr)
				}
			} else {
				result.WriteString("go vet: no issues found")
			}

			output := result.String()
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"path": args.Path},
			}, nil
		},
	}
}
