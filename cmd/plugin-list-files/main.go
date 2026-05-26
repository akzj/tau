package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
				"name":    "plugin-list-files",
				"version": "0.1.0",
				"tools": []map[string]any{{
					"name":        "plugin-list-files",
					"description": "List files and directories with depth control and glob filtering (external plugin)",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path":        map[string]any{"type": "string", "description": "Directory path (default: .)"},
							"depth":       map[string]any{"type": "integer", "description": "Recursion depth (1-5, default 1)"},
							"pattern":     map[string]any{"type": "string", "description": "Glob filter (e.g., *.go)"},
							"show_hidden": map[string]any{"type": "boolean", "description": "Show hidden files"},
						},
					},
				}},
			}
		case "execute":
			args, _ := req.Params["args"].(map[string]any)
			path := "."
			if p, ok := args["path"].(string); ok && p != "" {
				path = p
			}
			depth := 1
			if d, ok := args["depth"].(float64); ok && d > 0 && d <= 5 {
				depth = int(d)
			}
			pattern, _ := args["pattern"].(string)
			showHidden, _ := args["show_hidden"].(bool)

			var entries []string
			maxDepth := depth
			filepath.WalkDir(path, func(fp string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				rel, _ := filepath.Rel(path, fp)
				if rel == "." {
					return nil
				}
				depth := strings.Count(rel, string(os.PathSeparator)) + 1
				if depth > maxDepth {
					return filepath.SkipDir
				}
				if !showHidden && strings.HasPrefix(d.Name(), ".") {
					return nil
				}
				if pattern != "" {
					matched, _ := filepath.Match(pattern, d.Name())
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
			if len(entries) == 0 {
				resp = map[string]any{"result": "(empty directory)"}
			} else {
				resp = map[string]any{"result": strings.Join(entries, "\n")}
			}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}
