package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
				"name":    "plugin-run-tests",
				"version": "0.1.0",
				"tools": []map[string]any{{
					"name":        "plugin-run-tests",
					"description": "Run tests with auto-detected framework (Go/Python/JS/Rust) (external plugin)",
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"path":            map[string]any{"type": "string", "description": "Test directory path"},
							"framework":       map[string]any{"type": "string", "description": "go/py/js/rust/auto"},
							"timeout_seconds": map[string]any{"type": "integer", "description": "Timeout (default 30, max 120)"},
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
			framework := "auto"
			if f, ok := args["framework"].(string); ok && f != "" {
				framework = f
			}
			timeoutSec := 30
			if ts, ok := args["timeout_seconds"].(float64); ok && ts > 0 {
				timeoutSec = int(ts)
			}
			if timeoutSec > 120 {
				timeoutSec = 120
			}

			if framework == "auto" {
				framework = detectFramework(path)
			}
			result := runTests(path, framework, timeoutSec)
			resp = map[string]any{"result": result}
		default:
			resp = map[string]any{"error": "unknown method: " + req.Method}
		}
		out, _ := json.Marshal(resp)
		fmt.Println(string(out))
	}
}

func detectFramework(dir string) string {
	checks := []struct{ file, fw string }{
		{"go.mod", "go"}, {"pyproject.toml", "py"}, {"setup.py", "py"},
		{"package.json", "js"}, {"Cargo.toml", "rust"},
	}
	for _, c := range checks {
		if _, err := os.Stat(filepath.Join(dir, c.file)); err == nil {
			return c.fw
		}
	}
	return ""
}

func runTests(dir, framework string, timeoutSec int) string {
	if framework == "" {
		return "No test framework detected."
	}
	var cmd *exec.Cmd
	switch framework {
	case "go":
		cmd = exec.Command("go", "test", "-v", "-count=1", "./...")
	case "py":
		cmd = exec.Command("python", "-m", "pytest", "-v")
	case "js":
		cmd = exec.Command("npx", "jest", "--verbose")
	case "rust":
		cmd = exec.Command("cargo", "test")
	default:
		return "Unknown framework: " + framework
	}
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))
	out, err := cmd.Output()
	result := string(out)
	if err != nil {
		result += fmt.Sprintf("\n[exit: %v]", err)
	}
	if len(result) > 4000 {
		result = result[:4000] + "\n... (truncated)"
	}
	_ = timeoutSec
	return result
}
