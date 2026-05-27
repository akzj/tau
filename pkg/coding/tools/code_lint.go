package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// CodeLintTool creates a multi-linter runner tool.
//
// Parameters:
//
//	path    (string, required) — file or directory path to lint
//	linters (string, optional) — comma-separated linters (default: all enabled). Only applies when golangci-lint is used.
func CodeLintTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path to lint"},
			"linters": {"type": "string", "description": "Comma-separated linters (e.g., golint,vet,staticcheck)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "code_lint",
		Description: "Run linters on code (golangci-lint). Analyzes Go code for common issues and style violations.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string `json:"path"`
				Linters string `json:"linters"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			// Try golangci-lint first, fall back to go vet
			timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			// Try golangci-lint
			if _, err := exec.LookPath("golangci-lint"); err == nil {
				cmdArgs := []string{"run", "--timeout=60s"}
				if args.Linters != "" {
					cmdArgs = append(cmdArgs, "-E", args.Linters)
					cmdArgs = append(cmdArgs, "--disable-all")
				}
				cmdArgs = append(cmdArgs, args.Path)
				cmd := exec.CommandContext(timeoutCtx, "golangci-lint", cmdArgs...)
				cmd.Dir = WorkspaceRoot
				output, err := cmd.CombinedOutput()
				text := strings.TrimSpace(string(output))
				if err != nil && text == "" {
					return core.ToolResult{
						Content: []core.Content{{Type: "text", Text: fmt.Sprintf("golangci-lint failed: %v", err)}},
						Details: map[string]any{"path": args.Path, "linter": "golangci-lint", "success": false},
					}, err
				}
				if text == "" {
					text = "No issues found (golangci-lint)"
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: text}},
					Details: map[string]any{"path": args.Path, "linter": "golangci-lint", "success": err == nil},
				}, nil
			}

			// Fall back to go vet
			cmd := exec.CommandContext(timeoutCtx, "go", "vet", args.Path)
			cmd.Dir = WorkspaceRoot
			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))
			if err != nil && text == "" {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("go vet failed: %v", err)}},
					Details: map[string]any{"path": args.Path, "linter": "go vet", "success": false},
				}, err
			}
			if text == "" {
				text = "No issues found (go vet)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"path": args.Path, "linter": "go vet", "success": err == nil},
			}, nil
		},
	}
}
