package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		var req struct {
			Method string         `json:"method"`
			Params map[string]any `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			continue
		}

		var resp map[string]any
		switch req.Method {
		case "tools":
			resp = map[string]any{
				"name":    "plugin-search-code",
				"version": "0.1.0",
				"tools": []map[string]any{{
					"name":        "plugin-search-code",
					"description": "Search code with regex, file filter, and context lines (external plugin)",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"query":         map[string]any{"type": "string", "description": "Regex pattern"},
							"path":          map[string]any{"type": "string", "description": "Search directory"},
							"file_glob":     map[string]any{"type": "string", "description": "File filter glob"},
							"context_lines": map[string]any{"type": "integer", "description": "Context lines (default 1)"},
							"max_results":   map[string]any{"type": "integer", "description": "Max results (default 50)"},
						},
					},
					"required": []string{"query"},
				}},
			}
		case "execute":
			args, _ := req.Params["args"].(map[string]any)
			query, _ := args["query"].(string)
			if query == "" {
				resp = map[string]any{"error": "query required"}
				break
			}

			path := "."
			if p, ok := args["path"].(string); ok && p != "" {
				path = p
			}
			fileGlob, _ := args["file_glob"].(string)
			ctxLines := 1
			if cl, ok := args["context_lines"].(float64); ok && cl > 0 {
				ctxLines = int(cl)
			}
			maxResults := 50
			if mr, ok := args["max_results"].(float64); ok && mr > 0 {
				maxResults = int(mr)
			}

			re, err := regexp.Compile(query)
			if err != nil {
				resp = map[string]any{"error": "invalid regex: " + err.Error()}
				break
			}

			var results []string
			count := 0
			filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
				if err != nil || count >= maxResults {
					return nil
				}
				if d.IsDir() || strings.HasPrefix(d.Name(), ".") {
					return nil
				}
				if fileGlob != "" {
					if matched, _ := filepath.Match(fileGlob, d.Name()); !matched {
						return nil
					}
				}
				matches := searchFile(fp, re, ctxLines, maxResults-count)
				results = append(results, matches...)
				count += len(matches)
				return nil
			})
			if len(results) == 0 {
				resp = map[string]any{"result": "No matches found."}
			} else {
				resp = map[string]any{"result": strings.Join(results, "\n")}
			}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

func searchFile(path string, re *regexp.Regexp, ctxLines, maxResults int) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var results []string
	for i, line := range lines {
		if re.MatchString(line) {
			rel, _ := filepath.Rel(".", path)
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
