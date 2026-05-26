package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/akzj/tau/core"
)

// GitCommitTool creates a git commit tool.
func GitCommitTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"message": {"type": "string", "description": "Commit message"},
			"files": {"type": "array", "items": {"type": "string"}, "description": "Files to add (default: all modified)"}
		},
		"required": ["message"]
	}`)

	return core.Tool{
		Name:        "git_commit",
		Description: "Stage and commit changes. Uses git add + git commit. IsDangerous — requires confirmation.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Message string   `json:"message"`
				Files   []string `json:"files"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Message == "" {
				return core.ToolResult{}, fmt.Errorf("commit message required")
			}
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			// Stage files
			addArgs := []string{"add"}
			if len(args.Files) > 0 {
				addArgs = append(addArgs, args.Files...)
			} else {
				addArgs = append(addArgs, ".")
			}
			addCmd := exec.CommandContext(ctx, "git", addArgs...)
			addCmd.Dir = WorkspaceRoot
			if out, err := addCmd.CombinedOutput(); err != nil {
				return core.ToolResult{}, fmt.Errorf("git add: %s", string(out))
			}

			// Commit
			commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", args.Message)
			commitCmd.Dir = WorkspaceRoot
			var stdout, stderr bytes.Buffer
			commitCmd.Stdout = &stdout
			commitCmd.Stderr = &stderr
			err := commitCmd.Run()
			output := stdout.String()
			if stderr.Len() > 0 {
				output += stderr.String()
			}
			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("❌ Commit failed: %s", output)}},
				}, nil
			}
			// Extract hash
			hash := ""
			if idx := strings.Index(output, "]"); idx > 0 {
				hash = strings.TrimSpace(output[:idx])
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("✅ Committed: %s\n%s", hash, output)}},
				Details: map[string]any{"message": args.Message, "hash": hash},
			}, nil
		},
	}
}
