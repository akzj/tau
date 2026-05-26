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

// SearchCodeTool creates a regex code search tool.
func SearchCodeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Regex pattern to search for"},
			"path": {"type": "string", "description": "Directory or file path (default: workspace root)"},
			"file_glob": {"type": "string", "description": "File filter glob (e.g., *.go)"},
			"context_lines": {"type": "integer", "description": "Lines of context around match (default 1)"},
			"max_results": {"type": "integer", "description": "Maximum results (default 50)"}
		},
		"required": ["query"]
	}`)

	return core.Tool{
		Name:        "search_code",
		Description: "Search code with regex. Returns matching lines with file:line references and context.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Query        string `json:"query"`
				Path         string `json:"path"`
				FileGlob     string `json:"file_glob"`
				ContextLines int    `json:"context_lines"`
				MaxResults   int    `json:"max_results"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.ContextLines <= 0 {
				args.ContextLines = 1
			}
			if args.MaxResults <= 0 {
				args.MaxResults = 50
			}

			re, err := regexp.Compile(args.Query)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("invalid regex: %w", err)
			}

			searchDir := WorkspaceRoot
			if args.Path != "" {
				searchDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			var results []string
			count := 0
			filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
				if err != nil || count >= args.MaxResults {
					return nil
				}
				if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
					return nil
				}
				if args.FileGlob != "" {
					matched, _ := filepath.Match(args.FileGlob, d.Name())
					if !matched {
						return nil
					}
				}
				matches := searchFile(path, re, args.ContextLines, args.MaxResults-count)
				results = append(results, matches...)
				count += len(matches)
				return nil
			})

			if len(results) == 0 {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: "No matches found."}}}, nil
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: strings.Join(results, "\n")}},
				Details: map[string]any{"pattern": args.Query, "matches": len(results)},
			}, nil
		},
	}
}

func searchFile(path string, re *regexp.Regexp, ctxLines, maxResults int) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	var results []string
	for i, line := range lines {
		if re.MatchString(line) {
			rel, _ := filepath.Rel(WorkspaceRoot, path)
			start := i - ctxLines
			if start < 0 {
				start = 0
			}
			end := i + ctxLines + 1
			if end > len(lines) {
				end = len(lines)
			}
			for j := start; j < end; j++ {
				marker := "  "
				if j == i {
					marker = ">>"
				}
				results = append(results, fmt.Sprintf("%s:%d: %s%s", rel, j+1, marker, lines[j]))
			}
			results = append(results, "---")
			if len(results) >= maxResults*3 {
				return results[:maxResults*3]
			}
		}
	}
	return results
}
