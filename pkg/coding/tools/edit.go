package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// EditTool creates a find-and-replace editing tool.
func EditTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string"},
			"old": {"type": "string", "description": "Exact text to replace"},
			"new": {"type": "string", "description": "Replacement text"},
			"n": {"type": "integer", "description": "Max replacements (-1 = all, default: -1)"}
		},
		"required": ["file_path", "old", "new"]
	}`)

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Old      string `json:"old"`
				New      string `json:"new"`
				N        int    `json:"n"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}
			if args.N == 0 {
				args.N = -1
			}

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.PreparedTool{}, err
			}
			args.FilePath = path

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "edit",
				Params:   args,
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string
				Old      string
				New      string
				N        int
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)

			data, err := os.ReadFile(args.FilePath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("edit %s: read: %w", args.FilePath, err)
			}

			content := string(data)
			if !strings.Contains(content, args.Old) {
				preview := content
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				if preview == "" {
					preview = "(empty file)"
				}
				return core.ToolResult{}, fmt.Errorf("edit %s: old text not found (file preview: %q)", args.FilePath, preview)
			}

			// Backup with timestamp
			backupDir := filepath.Join(WorkspaceRoot, ".tau-backups")
			os.MkdirAll(backupDir, 0755)
			ts := time.Now().UTC().Format("20060102T150405")
			backupPath := filepath.Join(backupDir, fmt.Sprintf("%s.%s.bak", filepath.Base(args.FilePath), ts))
			if err := os.WriteFile(backupPath, data, 0644); err != nil {
				fmt.Fprintf(os.Stderr, "  [edit] backup write failed (non-fatal): %v\n", err)
			}

			replaced := strings.Replace(content, args.Old, args.New, args.N)
			if err := os.WriteFile(args.FilePath, []byte(replaced), 0644); err != nil {
				return core.ToolResult{}, fmt.Errorf("edit %s: write: %w", args.FilePath, err)
			}

			count := strings.Count(content, args.Old)
			if args.N > 0 && args.N < count {
				count = args.N
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Replaced %d occurrence(s) in %s", count, args.FilePath)}},
			}, nil
		},
	}

	return core.Tool{
		Name:        "edit",
		Description: "Find and replace text in a file. Creates .tau-backups/ before editing.",
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
