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
				"name":    "plugin-workspace-diag",
				"version": "0.1.0",
				"tools": []map[string]any{{
					"name":        "plugin-workspace-diag",
					"description": "Scan the workspace: file tree, git status, language statistics (external plugin)",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"depth":       map[string]any{"type": "integer", "description": "Directory depth (1-3)"},
							"include_git": map[string]any{"type": "boolean", "description": "Include git status"},
						},
					},
				}},
			}
		case "execute":
			args, _ := req.Params["args"].(map[string]any)
			depth := 2
			if d, ok := args["depth"].(float64); ok {
				depth = int(d)
			}
			if depth < 1 || depth > 3 {
				depth = 2
			}

			includeGit := true
			if ig, ok := args["include_git"].(bool); ok {
				includeGit = ig
			}

			result := runDiag(depth, includeGit)
			resp = map[string]any{"result": result}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}

		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

func runDiag(depth int, includeGit bool) string {
	workspace := "."
	if wd, err := os.Getwd(); err == nil {
		workspace = wd
	}

	fileTree := make(map[string]int)
	langStats := make(map[string]int)
	totalFiles := 0

	_ = includeGit // reserved for future use

	filepath.WalkDir(workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == workspace {
				return nil
			}
			rel, _ := filepath.Rel(workspace, path)
			if strings.Count(rel, string(os.PathSeparator)) >= depth {
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == "" {
			ext = "(no ext)"
		}
		fileTree[ext]++
		totalFiles++
		switch ext {
		case ".go":
			langStats["Go"]++
		case ".py", ".pyw":
			langStats["Python"]++
		case ".js", ".ts", ".jsx", ".tsx":
			langStats["JS/TS"]++
		case ".md", ".mdx":
			langStats["Markdown"]++
		case ".json", ".yaml", ".yml", ".toml":
			langStats["Config"]++
		default:
			langStats["Other"]++
		}
		return nil
	})

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## Workspace Diagnostic (plugin)\n\n**Total files**: %d\n\n", totalFiles))

	// Language breakdown (sorted by count)
	type kv struct {
		k string
		v int
	}
	var sorted []kv
	for k, v := range langStats {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	for _, kv := range sorted {
		if kv.v > 0 {
			b.WriteString(fmt.Sprintf("- %s: %d files (%.1f%%)\n", kv.k, kv.v, float64(kv.v)/float64(totalFiles)*100))
		}
	}

	// Extension breakdown
	var exts []kv
	for k, v := range fileTree {
		exts = append(exts, kv{k, v})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].v > exts[j].v })
	b.WriteString("\n### Extensions\n")
	for _, kv := range exts {
		if kv.v >= 1 {
			b.WriteString(fmt.Sprintf("- %s: %d\n", kv.k, kv.v))
		}
	}

	return b.String()
}
