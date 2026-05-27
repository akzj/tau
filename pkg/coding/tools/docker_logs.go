package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// DockerLogsTool creates a docker logs tool.
//
// Parameters:
//
//	container (string, required) — container name or ID
//	tail      (number, optional) — number of lines to show from the end (default: 100)
//	follow    (bool, optional) — follow log output (timeout-limited)
//	since     (string, optional) — show logs since timestamp (e.g., "10m", "1h")
//	timestamps (bool, optional) — show timestamps
func DockerLogsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"container": {"type": "string", "description": "Container name or ID"},
			"tail": {"type": "number", "description": "Number of lines to show from end (default: 100)"},
			"follow": {"type": "boolean", "description": "Follow log output (timeout-limited)"},
			"since": {"type": "string", "description": "Show logs since timestamp (e.g., 10m, 1h)"},
			"timestamps": {"type": "boolean", "description": "Show timestamps"}
		},
		"required": ["container"]
	}`)

	return core.Tool{
		Name:        "docker_logs",
		Description: "Retrieve logs from a Docker container. Supports tail, follow (timeout-limited), since, and timestamps.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Container  string  `json:"container"`
				Tail       float64 `json:"tail"`
				Follow     bool    `json:"follow"`
				Since      string  `json:"since"`
				Timestamps bool    `json:"timestamps"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Container == "" {
				return core.ToolResult{}, fmt.Errorf("container name or ID required")
			}

			if _, err := exec.LookPath("docker"); err != nil {
				return core.ToolResult{}, fmt.Errorf("docker not found in PATH")
			}

			cmdArgs := []string{"logs"}
			if args.Tail > 0 {
				cmdArgs = append(cmdArgs, "--tail", strconv.Itoa(int(args.Tail)))
			} else {
				cmdArgs = append(cmdArgs, "--tail", "100")
			}
			if args.Timestamps {
				cmdArgs = append(cmdArgs, "--timestamps")
			}
			if args.Since != "" {
				cmdArgs = append(cmdArgs, "--since", args.Since)
			}
			cmdArgs = append(cmdArgs, args.Container)

			timeout := 30 * time.Second
			if args.Follow {
				timeout = 10 * time.Second // Limit follow
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cmd := exec.CommandContext(timeoutCtx, "docker", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("docker logs failed:\n%s", text)}},
					Details: map[string]any{"container": args.Container, "success": false},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("No logs for container: %s", args.Container)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{"container": args.Container, "success": true},
			}, nil
		},
	}
}
