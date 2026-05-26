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
			"n": {"type": "integer", "description": "Max results (default 100)"},
			"context_lines": {"type": "integer", "description": "Show N lines before and after each match (default 0)"},
			"ignore_case": {"type": "boolean", "description": "Case-insensitive search (default false)"}
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
				Pattern      string `json:"pattern"`
				Path         string `json:"path"`
				Include      string `json:"include"`
				N            int    `json:"n"`
				ContextLines int    `json:"context_lines"`
				IgnoreCase   bool   `json:"ignore_case"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.N <= 0 {
				args.N = 100
			}

			pattern := args.Pattern
			if args.IgnoreCase {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("grep: invalid regex: %w", err)
			}

			searchPath, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			var results []string
			count := 0

			info, err := os.Stat(searchPath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("grep: stat %s: %w", args.Path, err)
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
					fileResults, skipped := grepFileEx(path, re, WorkspaceRoot, args.N-count, args.ContextLines)
					if skipped {
						rel, _ := filepath.Rel(WorkspaceRoot, path)
						results = append(results, fmt.Sprintf("%s: [binary file — skipped]", rel))
					}
					results = append(results, fileResults...)
					count += len(fileResults)
					return nil
				})
			} else {
				fileResults, skipped := grepFileEx(searchPath, re, WorkspaceRoot, args.N, args.ContextLines)
				if skipped {
					rel, _ := filepath.Rel(WorkspaceRoot, searchPath)
					results = append(results, fmt.Sprintf("%s: [binary file — skipped]", rel))
				}
				results = append(results, fileResults...)
			}

			if len(results) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("No matches found for pattern: %s", args.Pattern)}},
				}, nil
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: strings.Join(results, "\n")}},
			}, nil
		},
	}
}

// grepFileEx searches a file with context lines. Returns (results, binarySkipped).
func grepFileEx(path string, re *regexp.Regexp, root string, maxResults int, ctxLines int) ([]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	// Read all lines into buffer for context support
	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.ContainsRune(text, 0) {
			// Binary file — skip
			return nil, true
		}
		allLines = append(allLines, text)
	}

	rel, _ := filepath.Rel(root, path)

	var results []string
	printed := make(map[int]bool) // track which lines we've already printed

	for i, line := range allLines {
		if len(results) >= maxResults {
			break
		}
		if re.MatchString(line) {
			// Print separator between non-contiguous match groups
			start := i - ctxLines
			if start < 0 {
				start = 0
			}
			end := i + ctxLines
			if end >= len(allLines) {
				end = len(allLines) - 1
			}

			for j := start; j <= end; j++ {
				if printed[j] {
					continue
				}
				printed[j] = true
				marker := " "
				if re.MatchString(allLines[j]) {
					marker = ":"
				} else {
					marker = "-"
				}
				results = append(results, fmt.Sprintf("%s%s%d%s %s", rel, marker, j+1, marker, allLines[j]))
				if len(results) >= maxResults {
					break
				}
			}
		}
	}
	return results, false
}
