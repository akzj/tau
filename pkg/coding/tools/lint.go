package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/akzj/tau/core"
)

// LintTool creates a golangci-lint runner tool.
func LintTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path to lint (default: ./...)"},
			"fast": {"type": "boolean", "description": "Fast mode — only newly created issues (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "lint",
		Description: "Run golangci-lint and return structured issues (file, line, message, severity).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path string `json:"path"`
				Fast bool   `json:"fast"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = "./..."
			}
			cmdArgs := []string{"run", "--out-format=json", args.Path}
			if args.Fast {
				cmdArgs = []string{"run", "--new", "--out-format=json", args.Path}
			}
			cmd := exec.CommandContext(ctx, "golangci-lint", cmdArgs...)
			cmd.Dir = WorkspaceRoot
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}
			if strings.TrimSpace(output) == "" {
				output = "No issues found."
			}
			if err != nil && !strings.Contains(err.Error(), "exit status 1") {
				return core.ToolResult{}, fmt.Errorf("lint: %w", err)
			}
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
