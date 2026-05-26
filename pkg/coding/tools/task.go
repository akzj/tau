package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

	tp := &toolThreePhase{
		prepare: func(ctx context.Context, callID string, params any) (core.PreparedTool, error) {
			var args struct {
				Action string `json:"action"`
				TaskID string `json:"task_id"`
				Title  string `json:"title"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.PreparedTool{}, err
			}

			if args.Action == "" {
				args.Action = "list"
			}
			if args.Action == "create" && args.TaskID == "" {
				args.TaskID = fmt.Sprintf("task-%d", len(tasks)+1)
			}

			return core.PreparedTool{
				CallID:   callID,
				ToolName: "task",
				Params:   args,
				State:    tasks,
			}, nil
		},
		execute: func(ctx context.Context, prepared core.PreparedTool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string
				TaskID string
				Title  string
			}
			raw, _ := json.Marshal(prepared.Params)
			json.Unmarshal(raw, &args)
			tasks := prepared.State.(map[string]string)

			switch args.Action {
			case "create":
				if _, exists := tasks[args.TaskID]; exists {
					return core.ToolResult{}, fmt.Errorf("task: duplicate task_id %q — use update instead", args.TaskID)
				}
				tasks[args.TaskID] = args.Title
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Created task %s: %s", args.TaskID, args.Title)}},
				}, nil
			case "update":
				if args.TaskID == "" {
					return core.ToolResult{}, fmt.Errorf("task: task_id is required for update")
				}
				if _, exists := tasks[args.TaskID]; !exists {
					return core.ToolResult{}, fmt.Errorf("task: task_id %q not found — use create instead", args.TaskID)
				}
				tasks[args.TaskID] = args.Title
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Updated task %s: %s", args.TaskID, args.Title)}},
				}, nil
			case "complete":
				if args.TaskID == "" {
					return core.ToolResult{}, fmt.Errorf("task: task_id is required for complete")
				}
				if _, exists := tasks[args.TaskID]; !exists {
					return core.ToolResult{}, fmt.Errorf("task: task_id %q not found", args.TaskID)
				}
				delete(tasks, args.TaskID)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Completed task %s", args.TaskID)}},
				}, nil
			default:
				// List all tasks
				if len(tasks) == 0 {
					return core.ToolResult{
						Content: []core.Content{{Type: "text", Text: "No active tasks."}},
					}, nil
				}
				var keys []string
				for k := range tasks {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				var lines []string
				for i, id := range keys {
					lines = append(lines, fmt.Sprintf("%d. [ ] %s: %s", i+1, id, tasks[id]))
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: strings.Join(lines, "\n")}},
				}, nil
			}
		},
	}

	return core.Tool{
		Name:        "task",
		Description: "Manage a task list for the current session.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		ThreePhase:  tp,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			prepared, err := tp.Prepare(ctx, callID, params)
			if err != nil {
				return core.ToolResult{}, err
			}
			return tp.Execute(ctx, prepared, onUpdate)
		},
	}
}
