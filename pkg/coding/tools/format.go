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

// FormatTool creates a gofmt-based code formatter tool.
func FormatTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path (default: ./...)"},
			"check": {"type": "boolean", "description": "Check-only mode — return diff without modifying files (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "format",
		Description: "Format code with gofmt. Check mode shows diff without modifying files.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path  string `json:"path"`
				Check bool   `json:"check"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				args.Path = "./..."
			}
			var cmdArgs []string
			if args.Check {
				cmdArgs = append(cmdArgs, "-d", args.Path)
			} else {
				cmdArgs = append(cmdArgs, "-w", args.Path)
			}
			cmd := exec.CommandContext(ctx, "gofmt", cmdArgs...)
			cmd.Dir = WorkspaceRoot
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += "\n" + stderr.String()
			}
			if args.Check && strings.TrimSpace(output) == "" {
				output = "All files formatted correctly."
			}
			if !args.Check {
				output = fmt.Sprintf("Formatted %s", args.Path)
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"path": args.Path, "check": args.Check},
			}, nil
		},
	}
}
