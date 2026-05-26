package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// GrepTool creates a regex-search tool.
func GrepTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "File or directory to search (relative to workspace)"},
			"include": {"type": "string", "description": "File glob filter (e.g., *.go) (optional)"},
			"n": {"type": "integer", "description": "Max results (default 100)"}
		},
		"required": ["pattern", "path"]
	}`)

	return core.Tool{
		Name:        "grep",
		Description: "Search for a regex pattern in files.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Pattern string `json:"pattern"`
				Path    string `json:"path"`
				Include string `json:"include"`
				N       int    `json:"n"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.N <= 0 {
				args.N = 100
			}

			re, err := regexp.Compile(args.Pattern)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
			}

			searchPath, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			var results []string
			count := 0

			info, err := os.Stat(searchPath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("stat: %w", err)
			}

			if info.IsDir() {
				filepath.Walk(searchPath, func(path string, fi os.FileInfo, err error) error {
					if err != nil || count >= args.N {
						return nil
					}
					if fi.IsDir() {
						return nil
					}
					if args.Include != "" {
						matched, _ := filepath.Match(args.Include, filepath.Base(path))
						if !matched {
							return nil
						}
					}
					fileResults := grepFile(path, re, WorkspaceRoot, args.N-count)
					results = append(results, fileResults...)
					count += len(fileResults)
					return nil
				})
			} else {
				results = grepFile(searchPath, re, WorkspaceRoot, args.N)
			}

			if len(results) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "No matches found."}},
				}, nil
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: strings.Join(results, "\n")}},
			}, nil
		},
	}
}

func grepFile(path string, re *regexp.Regexp, root string, maxResults int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var results []string
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() && len(results) < maxResults {
		lineNum++
		if re.MatchString(scanner.Text()) {
			rel, _ := filepath.Rel(root, path)
			results = append(results, fmt.Sprintf("%s:%d: %s", rel, lineNum, scanner.Text()))
		}
	}
	return results
}
