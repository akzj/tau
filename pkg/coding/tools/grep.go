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

// GrepTool creates a regex search tool.
//
// Parameters:
//   pattern       (string, required) — regex pattern to search for
//   path          (string, required) — file or directory path to search
//   include       (string, optional) — file glob filter (e.g., "*.go")
//   context_lines (int, optional, default 0) — number of surrounding lines to show
//   ignore_case   (bool, optional, default false) — case-insensitive matching
//   n             (int, optional, default 100) — maximum number of results
//
// Returns matches in "file:line: content" format.
// Binary files are detected and skipped with a note.
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

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				Pattern      string `json:"pattern"`
				Path         string `json:"path"`
				Include      string `json:"include"`
				N            int    `json:"n"`
				ContextLines int    `json:"context_lines"`
				IgnoreCase   bool   `json:"ignore_case"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}
			if args.N <= 0 {
				args.N = 100
			}

			// Compile regex early to validate
			pattern := args.Pattern
			if args.IgnoreCase {
				pattern = "(?i)" + pattern
			}
			re, err := regexp.Compile(pattern)
			if err != nil {
				return core.PreparedTool{}, fmt.Errorf("grep: invalid regex: %w", err)
			}

			searchPath, err := ResolvePath(args.Path)
			if err != nil {
				return core.PreparedTool{}, err
			}

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "grep",
				Params:   args,
				State:    grepState{re: re, searchPath: searchPath},
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Pattern      string `json:"pattern"`
				Path         string `json:"path"`
				Include      string `json:"include"`
				N            int    `json:"n"`
				ContextLines int    `json:"context_lines"`
				IgnoreCase   bool   `json:"ignore_case"`
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)
			state := prepared.State.(grepState)

			var results []string
			count := 0

			info, err := os.Stat(state.searchPath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("grep: stat %s: %w", args.Path, err)
			}

			if info.IsDir() {
				filepath.Walk(state.searchPath, func(path string, fi os.FileInfo, err error) error {
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
					fileResults, skipped := grepFileEx(path, state.re, WorkspaceRoot, args.N-count, args.ContextLines)
					if skipped {
						rel, _ := filepath.Rel(WorkspaceRoot, path)
						results = append(results, fmt.Sprintf("%s: [binary file — skipped]", rel))
					}
					results = append(results, fileResults...)
					count += len(fileResults)
					return nil
				})
			} else {
				fileResults, skipped := grepFileEx(state.searchPath, state.re, WorkspaceRoot, args.N, args.ContextLines)
				if skipped {
					rel, _ := filepath.Rel(WorkspaceRoot, state.searchPath)
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

	return core.Tool{
		Name:        "grep",
		Description: "Search for a regex pattern in files.",
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

// grepState holds pre-compiled regex and resolved path for grep three-phase flow.
type grepState struct {
	re         *regexp.Regexp
	searchPath string
}

// grepFileEx searches a file with context lines. Returns (results, binarySkipped).
func grepFileEx(path string, re *regexp.Regexp, root string, maxResults int, ctxLines int) ([]string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.ContainsRune(text, 0) {
			return nil, true
		}
		allLines = append(allLines, text)
	}

	rel, _ := filepath.Rel(root, path)

	var results []string
	printed := make(map[int]bool)

	for i, line := range allLines {
		if len(results) >= maxResults {
			break
		}
		if re.MatchString(line) {
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
