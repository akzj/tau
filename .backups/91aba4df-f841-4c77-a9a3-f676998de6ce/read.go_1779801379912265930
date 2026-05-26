package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// ReadTool creates a read-file tool.
func ReadTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Path to file, relative to workspace root"},
			"offset": {"type": "integer", "description": "Line offset to start reading from (1-based, optional)"},
			"limit": {"type": "integer", "description": "Max lines to read (optional, default: all)"}
		},
		"required": ["file_path"]
	}`)

	return core.Tool{
		Name:        "read",
		Description: "Read a file from the workspace.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Offset   int    `json:"offset"`
				Limit    int    `json:"limit"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.ToolResult{}, err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read %s: %w", args.FilePath, err)
			}

			if len(data) > OutputCap {
				data = data[:OutputCap]
			}

			content := string(data)
			// Apply offset/limit if specified
			if args.Offset > 0 || args.Limit > 0 {
				lines := strings.Split(content, "\n")
				if args.Offset > 0 && args.Offset <= len(lines) {
					lines = lines[args.Offset-1:]
				}
				if args.Limit > 0 && args.Limit < len(lines) {
					lines = lines[:args.Limit]
				}
				content = strings.Join(lines, "\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: content}},
			}, nil
		},
	}
}
