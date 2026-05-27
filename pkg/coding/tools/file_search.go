package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// FileSearchTool creates a glob+regex file search tool.
//
// Parameters:
//
//	pattern (string, required) — glob pattern (e.g., "*.go", "**/*.md")
//	regex   (string, optional) — regex to filter file contents
//	path    (string, optional) — search root (default: workspace root)
//
// Uses filepath.Walk + regexp. No binary dependency.
// Results are capped at 200 files.
func FileSearchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Glob pattern for filenames (e.g., '*.go', '**/*.md')"},
			"regex": {"type": "string", "description": "Optional regex to match within file contents"},
			"path": {"type": "string", "description": "Search directory relative to workspace root (default: workspace root)"}
		},
		"required": ["pattern"]
	}`)

	return core.Tool{
		Name:        "file_search",
		Description: "Search files by glob pattern + optional content regex. Uses filepath.Walk, no binary deps.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Pattern string `json:"pattern"`
				Regex   string `json:"regex"`
				Path    string `json:"path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Pattern == "" {
				return core.ToolResult{}, fmt.Errorf("pattern required")
			}

			root := WorkspaceRoot
			if args.Path != "" {
				var err error
				root, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			var contentRe *regexp.Regexp
			if args.Regex != "" {
				var err error
				contentRe, err = regexp.Compile(args.Regex)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
				}
			}

			var matches []string
			var skipped []string

			filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
				}

				if err != nil {
					if os.IsPermission(err) {
						rel, _ := filepath.Rel(root, path)
						skipped = append(skipped, rel)
					}
					return nil
				}

				// Skip .git directories
				if info.IsDir() && info.Name() == ".git" {
					return filepath.SkipDir
				}

				if info.Mode()&os.ModeSymlink != 0 {
					return nil
				}

				if !info.IsDir() {
					rel, err := filepath.Rel(root, path)
					if err != nil {
						return nil
					}

					// Glob match on filename
					matched, err := filepath.Match(args.Pattern, filepath.Base(rel))
					if err != nil || !matched {
						return nil
					}

					// Regex match on content if specified
					if contentRe != nil {
						data, err := os.ReadFile(path)
						if err != nil || len(data) > 1024*1024 { // skip files > 1MB
							return nil
						}
						if !contentRe.Match(data) {
							return nil
						}
					}

					matches = append(matches, rel)
					if len(matches) >= 200 {
						return filepath.SkipAll
					}
				}
				return nil
			})

			output := fmt.Sprintf("## File Search: %s\n\n", args.Pattern)
			if len(matches) == 0 {
				output += "(no matches)"
			} else {
				for _, m := range matches {
					output += m + "\n"
				}
			}
			if len(matches) >= 200 {
				output += "\n... (capped at 200)"
			}
			if len(skipped) > 0 && len(skipped) <= 10 {
				output += fmt.Sprintf("\n\n[skipped: %s]", strings.Join(skipped, ", "))
			} else if len(skipped) > 10 {
				output += fmt.Sprintf("\n\n[skipped: %d directories]", len(skipped))
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"pattern": args.Pattern, "matches": len(matches)},
			}, nil
		},
	}
}
