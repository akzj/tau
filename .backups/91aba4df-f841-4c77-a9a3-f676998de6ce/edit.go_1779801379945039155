package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	return core.Tool{
		Name:        "edit",
		Description: "Find and replace text in a file. Creates .tau-backups/ before editing.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath string `json:"file_path"`
				Old      string `json:"old"`
				New      string `json:"new"`
				N        int    `json:"n"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.N == 0 {
				args.N = -1
			}

			path, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.ToolResult{}, err
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read: %w", err)
			}

			content := string(data)
			if !strings.Contains(content, args.Old) {
				return core.ToolResult{}, fmt.Errorf("old text not found in %s", args.FilePath)
			}

			// Backup
			backupDir := filepath.Join(WorkspaceRoot, ".tau-backups")
			os.MkdirAll(backupDir, 0755)
			backupPath := filepath.Join(backupDir, filepath.Base(args.FilePath)+".bak")
			os.WriteFile(backupPath, data, 0644)

			replaced := strings.Replace(content, args.Old, args.New, args.N)
			if err := os.WriteFile(path, []byte(replaced), 0644); err != nil {
				return core.ToolResult{}, fmt.Errorf("write: %w", err)
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
}
