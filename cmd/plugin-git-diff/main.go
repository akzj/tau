package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
				"name":    "plugin-git-diff",
				"version": "0.1.0",
				"tools": []map[string]any{{
					"name":        "plugin-git-diff",
					"description": "Show git changes: unstaged (default), staged with --staged flag (external plugin)",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"staged": map[string]any{"type": "boolean"},
							"path":   map[string]any{"type": "string"},
						},
					},
				}},
			}
		case "execute":
			args, _ := req.Params["args"].(map[string]any)
			staged := false
			if s, ok := args["staged"].(bool); ok {
				staged = s
			}
			path := ""
			if p, ok := args["path"].(string); ok {
				path = p
			}

			cmdArgs := []string{"diff"}
			if staged {
				cmdArgs = append(cmdArgs, "--staged")
			}
			if path != "" {
				cmdArgs = append(cmdArgs, "--", path)
			}

			cmd := exec.Command("git", cmdArgs...)
			out, err := cmd.Output()
			result := string(out)
			if err != nil {
				result += fmt.Sprintf("\n[git error: %v]", err)
			}
			if result == "" {
				result = "(no changes)"
			}
			if len(result) > 4000 {
				result = result[:4000] + "\n... (truncated)"
			}

			resp = map[string]any{"result": result}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}
