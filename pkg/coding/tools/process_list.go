package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// ProcessListTool creates a cross-platform process listing tool.
//
// Parameters:
//
//	filter (string, optional) — filter processes by name substring
func ProcessListTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"filter": {"type": "string", "description": "Filter processes by name substring"}
		},
		"required": []
	}`)

	return core.Tool{
		Name:        "process_list",
		Description: "List running processes cross-platform (ps aux on Linux/macOS, tasklist on Windows). Supports name filtering.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Filter string `json:"filter"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()

			var cmd *exec.Cmd
			if runtime.GOOS == "windows" {
				cmd = exec.CommandContext(timeoutCtx, "tasklist")
			} else {
				cmd = exec.CommandContext(timeoutCtx, "ps", "aux")
			}
			output, err := cmd.CombinedOutput()
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("process list: %w", err)
			}

			text := string(output)
			if args.Filter != "" {
				lines := strings.Split(text, "\n")
				var filtered []string
				// Always keep header (first line)
				if len(lines) > 0 {
					filtered = append(filtered, lines[0])
				}
				for _, line := range lines[1:] {
					if strings.Contains(strings.ToLower(line), strings.ToLower(args.Filter)) {
						filtered = append(filtered, line)
					}
				}
				text = strings.Join(filtered, "\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"filter": args.Filter, "success": true},
			}, nil
		},
	}
}
