package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// FindReferencesTool creates a tool that finds references to a symbol in Go source files.
func FindReferencesTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"symbol": {"type": "string", "description": "Symbol name to find references for"},
			"path": {"type": "string", "description": "Directory path (default: workspace root)"}
		},
		"required": ["symbol"]
	}`)

	return core.Tool{
		Name:        "find_references",
		Description: "Find references to a symbol in Go source files using regex.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Symbol string `json:"symbol"`
				Path   string `json:"path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Symbol == "" {
				return core.ToolResult{}, fmt.Errorf("symbol required")
			}
			if args.Path == "" {
				args.Path = WorkspaceRoot
			}

			pattern := fmt.Sprintf(`\b%s\b`, regexp.QuoteMeta(args.Symbol))
			cmd := exec.CommandContext(ctx, "grep", "-rn", "--include=*.go", "-e", pattern, args.Path)
			out, err := cmd.Output()

			output := strings.TrimSpace(string(out))
			if output == "" || err != nil {
				output = fmt.Sprintf("No references found for '%s'", args.Symbol)
			}
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"symbol": args.Symbol},
			}, nil
		},
	}
}
