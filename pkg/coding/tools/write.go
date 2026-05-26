package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/akzj/tau/core"
)

// WriteTool creates a write-file tool.
func WriteTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "Path to file, relative to workspace root"},
			"content": {"type": "string", "description": "Content to write"}
		},
		"required": ["file_path", "content"]
	}`)

	return core.Tool{
		Name:        "write",
		Description: "Write content to a file. Creates parent directories if needed.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Content  string `json:"content"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.ToolResult{}, err
			}

			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return core.ToolResult{}, fmt.Errorf("mkdir: %w", err)
			}

			if err := os.WriteFile(path, []byte(args.Content), 0644); err != nil {
				return core.ToolResult{}, fmt.Errorf("write: %w", err)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath)}},
			}, nil
		},
	}
}
