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

// DockerPSTool creates a docker ps tool.
//
// Parameters:
//
//	filter (string, optional) — docker filter expression (e.g., "status=running")
//	format (string, optional) — output format (e.g., "table", "json")
//	all    (bool, optional) — show all containers (including stopped)
func DockerPSTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"filter": {"type": "string", "description": "Docker filter expression (e.g., status=running)"},
			"format": {"type": "string", "description": "Output format: table, json (default: table)"},
			"all": {"type": "boolean", "description": "Show all containers including stopped"}
		},
		"required": []
	}`)

	return core.Tool{
		Name:        "docker_ps",
		Description: "List Docker containers with filtering and formatting options. Wraps 'docker ps' command.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Filter string `json:"filter"`
				Format string `json:"format"`
				All    bool   `json:"all"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if _, err := exec.LookPath("docker"); err != nil {
				return core.ToolResult{}, fmt.Errorf("docker not found in PATH")
			}

			cmdArgs := []string{"ps"}
			if args.All {
				cmdArgs = append(cmdArgs, "-a")
			}
			if args.Filter != "" {
				cmdArgs = append(cmdArgs, "--filter", args.Filter)
			}
			if args.Format != "" {
				cmdArgs = append(cmdArgs, "--format", args.Format)
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(timeoutCtx, "docker", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("docker ps failed:\n%s", text)}},
					Details: map[string]any{"success": false},
				}, err
			}

			if text == "" {
				text = "No containers found"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"filter": args.Filter, "success": true},
			}, nil
		},
	}
}
