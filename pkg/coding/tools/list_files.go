package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// ListFilesTool creates a directory listing tool.
func ListFilesTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path relative to workspace root (default: root)"},
			"depth": {"type": "integer", "description": "Recursion depth (1-5, default 1)"},
			"pattern": {"type": "string", "description": "Glob pattern filter (e.g., *.go)"},
			"show_hidden": {"type": "boolean", "description": "Show hidden files (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "list_files",
		Description: "List files and directories. Supports recursion, glob filtering, and hidden file display.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path       string `json:"path"`
				Depth      int    `json:"depth"`
				Pattern    string `json:"pattern"`
				ShowHidden bool   `json:"show_hidden"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Depth <= 0 || args.Depth > 5 {
				args.Depth = 1
			}
			dir := WorkspaceRoot
			if args.Path != "" {
				var err error
				dir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}
			var entries []string
			maxDepth := args.Depth
			filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(dir, path)
				if rel == "." {
					return nil
				}
				depth := strings.Count(rel, string(os.PathSeparator)) + 1
				if depth > maxDepth {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if !args.ShowHidden && strings.HasPrefix(d.Name(), ".") {
					if d.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
				if args.Pattern != "" {
					matched, _ := filepath.Match(args.Pattern, d.Name())
					if !matched && !d.IsDir() {
						return nil
					}
				}
				info, _ := d.Info()
				typ := "file"
				if d.IsDir() {
					typ = "dir"
				}
				size := ""
				if info != nil && !d.IsDir() {
					size = fmt.Sprintf(" (%d)", info.Size())
				}
				entries = append(entries, fmt.Sprintf("%s [%s]%s", rel, typ, size))
				return nil
			})
			sort.Strings(entries)
			if len(entries) > 200 {
				entries = entries[:200]
			}
			text := strings.Join(entries, "\n")
			if text == "" {
				text = "(empty directory)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"path": args.Path, "count": len(entries)},
			}, nil
		},
	}
}
