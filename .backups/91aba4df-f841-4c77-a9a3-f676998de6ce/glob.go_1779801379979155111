package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// GlobTool creates a file-globbing tool.
func GlobTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern (e.g., **/*.go)"},
			"work_dir": {"type": "string", "description": "Search directory relative to workspace root (optional)"}
		},
		"required": ["pattern"]
	}`)

	return core.Tool{
		Name:        "glob",
		Description: "Find files matching a glob pattern.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Pattern string `json:"pattern"`
				WorkDir string `json:"work_dir"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			root := WorkspaceRoot
			if args.WorkDir != "" {
				var err error
				root, err = ResolvePath(args.WorkDir)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			var matches []string

			// Walk for ** patterns; fall back to filepath.Glob for simpler patterns
			if strings.Contains(args.Pattern, "**") {
				filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if err != nil {
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
				text = "(no matches)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
			}, nil
		},
	}
}
