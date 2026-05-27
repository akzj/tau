package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// EnvValidateTool creates a dependency/config validation tool.
//
// Parameters:
//
//	action (string, required) — check-deps | go | check-config | check-ports
//	path   (string, optional) — workspace path (default: workspace root)
//
// Actions:
//   - check-deps: verify external tools (git, go, docker, python, node)
//   - go: run "go mod verify"
//   - check-config: parse YAML/JSON config files for validity
//   - check-ports: check if required ports are available
func EnvValidateTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "Action: check-deps, go, check-config, check-ports"},
			"path": {"type": "string", "description": "Workspace path (default: workspace root)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "env_validate",
		Description: "Validate environment: check external deps, go mod verify, config parse, port availability.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Path   string `json:"path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Action == "" {
				return core.ToolResult{}, fmt.Errorf("action required (check-deps/go/check-config/check-ports)")
			}

			workDir := WorkspaceRoot
			if args.Path != "" {
				var err error
				workDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			switch args.Action {
			case "check-deps":
				return checkDeps(ctx, workDir)
			case "go":
				return checkGoMod(ctx, workDir)
			case "check-config":
				return checkConfig(workDir)
			case "check-ports":
				return checkPorts()
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use check-deps/go/check-config/check-ports)", args.Action)
			}
		},
	}
}

func checkDeps(ctx context.Context, workDir string) (core.ToolResult, error) {
	depTools := map[string][]string{
		"git":    {"git", "--version"},
		"go":     {"go", "version"},
		"docker": {"docker", "--version"},
		"python": {"python3", "--version"},
		"node":   {"node", "--version"},
	}

	var output strings.Builder
	output.WriteString("## Dependency Check\n\n")
	allOK := true

	for name, cmdParts := range depTools {
		cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
		cmd.Dir = workDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			output.WriteString(fmt.Sprintf("❌ %s: NOT FOUND (%v)\n", name, err))
			allOK = false
		} else {
			output.WriteString(fmt.Sprintf("✅ %s: %s\n", name, strings.TrimSpace(string(out))))
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output.String()}},
		Details: map[string]any{"all_ok": allOK},
	}, nil
}

func checkGoMod(ctx context.Context, workDir string) (core.ToolResult, error) {
	cmd := exec.CommandContext(ctx, "go", "mod", "verify")
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))

	if err != nil {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("## Go Mod Verify\n\n❌ FAIL\n\n%s", text)}},
			Details: map[string]any{"valid": false},
		}, nil // Don't return error — report in content
	}

	if text == "" {
		text = "✅ all modules verified"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("## Go Mod Verify\n\n%s", text)}},
		Details: map[string]any{"valid": true},
	}, nil
}

func checkConfig(workDir string) (core.ToolResult, error) {
	var output strings.Builder
	output.WriteString("## Config Validation\n\n")

	count := 0
	validCount := 0

	// Check for YAML and JSON config files
	checkExts := []string{".yaml", ".yml", ".json"}

	for _, ext := range checkExts {
		pattern := filepath.Join(workDir, "*"+ext)
		matches, _ := filepath.Glob(pattern)
		for _, f := range matches {
			// Skip node_modules and vendor
			if strings.Contains(f, "node_modules") || strings.Contains(f, "vendor") {
				continue
			}
			count++
			data, err := os.ReadFile(f)
			if err != nil {
				output.WriteString(fmt.Sprintf("❌ %s: cannot read (%v)\n", filepath.Base(f), err))
				continue
			}

			if ext == ".json" {
				var v any
				if err := json.Unmarshal(data, &v); err != nil {
					output.WriteString(fmt.Sprintf("❌ %s: invalid JSON (%v)\n", filepath.Base(f), err))
				} else {
					output.WriteString(fmt.Sprintf("✅ %s: valid JSON (%d bytes)\n", filepath.Base(f), len(data)))
					validCount++
				}
			} else {
				// Basic YAML validation: check for common issues
				lines := strings.Split(string(data), "\n")
				hasIssue := false
				for i, line := range lines {
					if strings.Contains(line, "\t") {
						output.WriteString(fmt.Sprintf("⚠️ %s:%d: tab character found\n", filepath.Base(f), i+1))
						hasIssue = true
					}
				}
				if !hasIssue {
					output.WriteString(fmt.Sprintf("✅ %s: looks valid (%d bytes)\n", filepath.Base(f), len(data)))
					validCount++
				}
			}
		}
	}

	if count == 0 {
		output.WriteString("(no config files found)")
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output.String()}},
		Details: map[string]any{"checked": count, "valid": validCount},
	}, nil
}

func checkPorts() (core.ToolResult, error) {
	ports := []int{8080, 3000, 8000, 5432, 6379}
	names := []string{"HTTP-8080", "Dev-3000", "API-8000", "Postgres-5432", "Redis-6379"}

	var output strings.Builder
	output.WriteString("## Port Check\n\n")

	for i, port := range ports {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
		if err != nil {
			output.WriteString(fmt.Sprintf("✅ %s (%d): available\n", names[i], port))
		} else {
			conn.Close()
			output.WriteString(fmt.Sprintf("⚠️ %s (%d): IN USE\n", names[i], port))
		}
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output.String()}},
	}, nil
}
