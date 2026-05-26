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
			"limit": {"type": "integer", "description": "Max lines to read (optional, default: all)"},
			"max_bytes": {"type": "integer", "description": "Max bytes to read (default 64KiB, max 1MiB)"}
		},
		"required": ["file_path"]
	}`)

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Offset   int    `json:"offset"`
				Limit    int    `json:"limit"`
				MaxBytes int    `json:"max_bytes"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.PreparedTool{}, err
			}
			args.FilePath = path // store resolved path

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "read",
				Params:   args,
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string
				Offset   int
				Limit    int
				MaxBytes int
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)

			if args.MaxBytes <= 0 || args.MaxBytes > OutputCap {
				args.MaxBytes = OutputCap
			}
			const maxLimit = 1024 * 1024 // 1MiB hard cap
			if args.MaxBytes > maxLimit {
				args.MaxBytes = maxLimit
			}

			data, err := os.ReadFile(args.FilePath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read %s: file not found (workspace: %s)", args.FilePath, WorkspaceRoot)
			}

			if len(data) > args.MaxBytes {
				data = data[:args.MaxBytes]
			}

			content := string(data)
			if strings.ContainsRune(content, 0) {
				content = "[binary file detected — showing text preview]\n" + content
			}

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

			if content == "" {
				content = "(empty file)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: content}},
			}, nil
		},
	}

	return core.Tool{
		Name:        "read",
		Description: "Read a file from the workspace.",
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
