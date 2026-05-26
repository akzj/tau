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

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Content  string `json:"content"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.PreparedTool{}, err
			}
			args.FilePath = path

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "write",
				Params:   args,
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string
				Content  string
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)

			overwritten := false
			if _, err := os.Stat(args.FilePath); err == nil {
				overwritten = true
			}

			if err := os.MkdirAll(filepath.Dir(args.FilePath), 0755); err != nil {
				return core.ToolResult{}, fmt.Errorf("write %s: cannot create parent dirs: %w", args.FilePath, err)
			}

			if err := os.WriteFile(args.FilePath, []byte(args.Content), 0644); err != nil {
				return core.ToolResult{}, fmt.Errorf("write %s: %w", args.FilePath, err)
			}

			msg := fmt.Sprintf("Wrote %d bytes to %s", len(args.Content), args.FilePath)
			if overwritten {
				msg += " (overwritten)"
			}
			if args.Content == "" {
				msg += " (empty — file cleared)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: msg}},
			}, nil
		},
	}

	return core.Tool{
		Name:        "write",
		Description: "Write content to a file. Creates parent directories if needed.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		ThreePhase:  tp,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			prepared, err := tp.Prepare(ctx, callID, params)
			if err != nil {
				return core.ToolResult{}, err
			}
			return tp.Execute(ctx, prepared, onUpdate)
		},
	}
}
