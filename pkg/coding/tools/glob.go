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

// GlobTool creates a file pattern matching tool.
//
// Parameters:
//   pattern  (string, required) — glob pattern (e.g., "**/*.go", "*.md")
//   work_dir (string, optional, default: workspace root) — search root directory
//   max_depth (int, optional, default 10) — maximum directory depth for ** patterns
//
// Returns matching file paths relative to workspace root.
// Results are capped at 200 matches. Symlinks are skipped with a note.
func GlobTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern (e.g., **/*.go)"},
			"work_dir": {"type": "string", "description": "Search directory relative to workspace root (optional)"},
			"max_depth": {"type": "integer", "description": "Max directory depth for ** patterns (default 10)"}
		},
		"required": ["pattern"]
	}`)

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				Pattern  string `json:"pattern"`
				WorkDir  string `json:"work_dir"`
				MaxDepth int    `json:"max_depth"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}
			if args.MaxDepth <= 0 {
				args.MaxDepth = 10
			}

			root := WorkspaceRoot
			if args.WorkDir != "" {
				var err error
				root, err = ResolvePath(args.WorkDir)
				if err != nil {
					return core.PreparedTool{}, err
				}
			}

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "glob",
				Params:   args,
				State:    root,
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Pattern  string `json:"pattern"`
				WorkDir  string `json:"work_dir"`
				MaxDepth int    `json:"max_depth"`
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)
			root := prepared.State.(string)

			var matches []string
			var permSkipped []string
			var symSkipped []string

			if strings.Contains(args.Pattern, "**") {
				baseDepth := strings.Count(filepath.Clean(root), string(filepath.Separator))
				filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
						if os.IsPermission(err) {
							rel, _ := filepath.Rel(root, path)
							permSkipped = append(permSkipped, rel)
						}
						return nil
					}

					depth := strings.Count(filepath.Clean(path), string(filepath.Separator)) - baseDepth
					if depth > args.MaxDepth {
						if info.IsDir() {
							return filepath.SkipDir
						}
						return nil
					}

					if info.Mode()&os.ModeSymlink != 0 {
						rel, _ := filepath.Rel(root, path)
						symSkipped = append(symSkipped, rel)
						return nil
					}

					rel, err := filepath.Rel(root, path)
					if err != nil {
						return nil
					}
					matched, err := filepath.Match(args.Pattern, rel)
					if err != nil {
						return nil
					}
					if matched {
						matches = append(matches, rel)
					}
					return nil
				})
			} else {
				pattern := filepath.Join(root, args.Pattern)
				found, err := filepath.Glob(pattern)
				if err != nil {
					return core.ToolResult{}, err
				}
				for _, m := range found {
					info, err := os.Lstat(m)
					if err != nil {
						continue
					}
					if info.Mode()&os.ModeSymlink != 0 {
						symSkipped = append(symSkipped, m)
						continue
					}
					rel, err := filepath.Rel(WorkspaceRoot, m)
					if err != nil {
						matches = append(matches, m)
					} else {
						matches = append(matches, rel)
					}
				}
			}

			if len(matches) > 200 {
				matches = matches[:200]
			}

			text := strings.Join(matches, "\n")
			if text == "" {
				text = fmt.Sprintf("(no matches for pattern: %s)", args.Pattern)
			}

			if len(permSkipped) > 0 {
				text += fmt.Sprintf("\n\n[skipped (permission): %s]", strings.Join(permSkipped, ", "))
			}
			if len(symSkipped) > 0 {
				text += fmt.Sprintf("\n\n[skipped (symlink): %s]", strings.Join(symSkipped, ", "))
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
			}, nil
		},
	}

	return core.Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern.",
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
