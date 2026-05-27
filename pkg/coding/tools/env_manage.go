package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// EnvManageTool creates an environment variable and .env file management tool.
func EnvManageTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["get", "set", "delete", "list"], "description": "CRUD operation"},
			"key": {"type": "string", "description": "Environment variable key"},
			"value": {"type": "string", "description": "Value (for set)"},
			"file": {"type": "string", "description": ".env file path (default: workspace/.env)"}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "env_manage",
		Description: "Manage environment variables and .env files (get/set/delete/list).",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Key    string `json:"key"`
				Value  string `json:"value"`
				File   string `json:"file"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.File == "" {
				args.File = filepath.Join(WorkspaceRoot, ".env")
			}

			switch args.Action {
			case "list":
				data, err := os.ReadFile(args.File)
				if os.IsNotExist(err) {
					return core.ToolResult{Content: []core.Content{{Type: "text", Text: "(no .env file)"}}}, nil
				}
				if err != nil {
					return core.ToolResult{}, err
				}
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: string(data)}}}, nil

			case "get":
				if args.Key == "" {
					return core.ToolResult{}, fmt.Errorf("key required for get")
				}
				val := os.Getenv(args.Key)
				if val == "" {
					return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("%s=(not set)", args.Key)}}}, nil
				}
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("%s=%s", args.Key, val)}}}, nil

			case "set":
				if args.Key == "" || args.Value == "" {
					return core.ToolResult{}, fmt.Errorf("key and value required for set")
				}
				content, _ := os.ReadFile(args.File)
				lines := strings.Split(string(content), "\n")
				// Filter empty trailing lines for clean append
				var cleanLines []string
				for _, l := range lines {
					if strings.TrimSpace(l) != "" {
						cleanLines = append(cleanLines, l)
					}
				}
				found := false
				for i, line := range cleanLines {
					if strings.HasPrefix(strings.TrimSpace(line), args.Key+"=") {
						cleanLines[i] = fmt.Sprintf("%s=%s", args.Key, args.Value)
						found = true
						break
					}
				}
				if !found {
					cleanLines = append(cleanLines, fmt.Sprintf("%s=%s", args.Key, args.Value))
				}
				err := os.WriteFile(args.File, []byte(strings.Join(cleanLines, "\n")+"\n"), 0644)
				if err != nil {
					return core.ToolResult{}, err
				}
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Set %s=%s", args.Key, args.Value)}}}, nil

			case "delete":
				if args.Key == "" {
					return core.ToolResult{}, fmt.Errorf("key required for delete")
				}
				content, _ := os.ReadFile(args.File)
				lines := strings.Split(string(content), "\n")
				var newLines []string
				for _, line := range lines {
					if !strings.HasPrefix(strings.TrimSpace(line), args.Key+"=") {
						newLines = append(newLines, line)
					}
				}
				os.WriteFile(args.File, []byte(strings.Join(newLines, "\n")), 0644)
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Deleted %s", args.Key)}}}, nil

			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s", args.Action)
			}
		},
	}
}
