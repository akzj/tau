package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/akzj/tau/core"
)

// TaskEntry is a tracked task.
type TaskEntry struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"` // "pending", "in_progress", "completed", "cancelled"
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// TaskTracker holds session-scoped tasks.
type TaskTracker struct {
	mu    sync.Mutex
	tasks map[string]TaskEntry
	order []string // insertion order
}

// GlobalTaskTracker is the session-scoped tracker (set by session init).
var GlobalTaskTracker = &TaskTracker{tasks: make(map[string]TaskEntry)}

// ResetGlobalTaskTracker resets the global tracker (for tests).
func ResetGlobalTaskTracker() {
	GlobalTaskTracker = &TaskTracker{tasks: make(map[string]TaskEntry)}
}

// Create adds a new task.
func (tt *TaskTracker) Create(title string) TaskEntry {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	id := fmt.Sprintf("task-%d", len(tt.tasks)+1)
	entry := TaskEntry{
		ID: id, Title: title, Status: "pending",
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	tt.tasks[id] = entry
	tt.order = append(tt.order, id)
	return entry
}

// Update modifies a task's status and/or title.
func (tt *TaskTracker) Update(id, status, title string) (TaskEntry, error) {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	entry, ok := tt.tasks[id]
	if !ok {
		return TaskEntry{}, fmt.Errorf("task %s not found", id)
	}
	if status != "" {
		entry.Status = status
	}
	if title != "" {
		entry.Title = title
	}
	entry.UpdatedAt = time.Now().Format(time.RFC3339)
	tt.tasks[id] = entry
	return entry, nil
}

// List returns all tasks in insertion order.
func (tt *TaskTracker) List() []TaskEntry {
	tt.mu.Lock()
	defer tt.mu.Unlock()
	var result []TaskEntry
	for _, id := range tt.order {
		result = append(result, tt.tasks[id])
	}
	return result
}

// TaskTrackerTool creates a task tracking tool.
func TaskTrackerTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["create", "update", "list"]},
			"task_id": {"type": "string"},
			"title": {"type": "string"},
			"status": {"type": "string", "enum": ["pending", "in_progress", "completed", "cancelled"]}
		},
		"required": ["action"]
	}`)

	return core.Tool{
		Name:        "task_tracker",
		Description: "Track tasks: create, update status, list all. Session-scoped in-memory storage.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				TaskID string `json:"task_id"`
				Title  string `json:"title"`
				Status string `json:"status"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			tt := GlobalTaskTracker

			switch args.Action {
			case "create":
				if args.Title == "" {
					return core.ToolResult{}, fmt.Errorf("title required for create")
				}
				entry := tt.Create(args.Title)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Created task %s: %s [pending]", entry.ID, entry.Title)}},
					Details: map[string]any{"task_id": entry.ID, "action": "create"},
				}, nil
			case "update":
				if args.TaskID == "" {
					return core.ToolResult{}, fmt.Errorf("task_id required for update")
				}
				entry, err := tt.Update(args.TaskID, args.Status, args.Title)
				if err != nil {
					return core.ToolResult{}, err
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Updated task %s: %s [%s]", entry.ID, entry.Title, entry.Status)}},
					Details: map[string]any{"task_id": entry.ID, "action": "update", "status": entry.Status},
				}, nil
			case "list":
				entries := tt.List()
				if len(entries) == 0 {
					return core.ToolResult{Content: []core.Content{{Type: "text", Text: "No tasks."}}}, nil
				}
				sort.Slice(entries, func(i, j int) bool { return entries[i].CreatedAt < entries[j].CreatedAt })
				var lines []string
				for _, e := range entries {
					statusIcon := "[ ]"
					switch e.Status {
					case "in_progress":
						statusIcon = "[▸]"
					case "completed":
						statusIcon = "[✓]"
					case "cancelled":
						statusIcon = "[✗]"
					}
					lines = append(lines, fmt.Sprintf("%s %s: %s", statusIcon, e.ID, e.Title))
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: strings.Join(lines, "\n")}},
					Details: map[string]any{"action": "list", "count": len(entries)},
				}, nil
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s", args.Action)
			}
		},
	}
}
