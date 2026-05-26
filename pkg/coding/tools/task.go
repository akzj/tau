package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// TaskTool creates a session-scoped task manager tool.
func TaskTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["create", "update", "complete", "list"]},
			"task_id": {"type": "string"},
			"title": {"type": "string"}
		},
		"required": ["action"]
	}`)

	// In-memory task store (session-scoped via closure)
	tasks := make(map[string]string)

	return core.Tool{
		Name:        "task",
		Description: "Manage a task list for the current session.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				TaskID string `json:"task_id"`
				Title  string `json:"title"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			switch args.Action {
			case "create":
				if args.TaskID == "" {
					args.TaskID = fmt.Sprintf("task-%d", len(tasks)+1)
				}
				tasks[args.TaskID] = args.Title
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Created task %s: %s", args.TaskID, args.Title)}},
				}, nil
			case "update":
				tasks[args.TaskID] = args.Title
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Updated task %s: %s", args.TaskID, args.Title)}},
				}, nil
			case "complete":
				delete(tasks, args.TaskID)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Completed task %s", args.TaskID)}},
				}, nil
			default:
				// List all tasks
				if len(tasks) == 0 {
					return core.ToolResult{
						Content: []core.Content{{Type: "text", Text: "(no tasks)"}},
					}, nil
				}
				var lines []string
				for id, title := range tasks {
					lines = append(lines, fmt.Sprintf("%s: %s", id, title))
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: strings.Join(lines, "\n")}},
				}, nil
			}
		},
	}
}
