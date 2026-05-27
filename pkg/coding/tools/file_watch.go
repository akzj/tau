package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/akzj/tau/core"
	"github.com/fsnotify/fsnotify"
)

// FileWatchTool creates a file system watch tool using fsnotify.
//
// Parameters:
//
//	path   (string, required) — directory or file path to watch
//	events (string, optional) — comma-separated events: create,write,remove,rename,chmod (default: all)
//	timeout (number, optional) — seconds to watch (default: 10, max: 60)
func FileWatchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory or file path to watch"},
			"events": {"type": "string", "description": "Comma-separated events: create,write,remove,rename,chmod (default: all)"},
			"timeout": {"type": "number", "description": "Seconds to watch (default: 10, max: 60)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "file_watch",
		Description: "Watch a directory or file for changes (create, write, remove, rename, chmod) using fsnotify.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string  `json:"path"`
				Events  string  `json:"events"`
				Timeout float64 `json:"timeout"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			timeout := 10 * time.Second
			if args.Timeout > 0 {
				timeout = time.Duration(args.Timeout * float64(time.Second))
				if timeout > 60*time.Second {
					timeout = 60 * time.Second
				}
			}

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("create watcher: %w", err)
			}
			defer watcher.Close()

			if err := watcher.Add(args.Path); err != nil {
				return core.ToolResult{}, fmt.Errorf("watch path: %w", err)
			}

			eventFilter := parseEventFilter(args.Events)

			var events []string
			timer := time.NewTimer(timeout)
			defer timer.Stop()

		loop:
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						break loop
					}
					if eventFilter[event.Op] || len(eventFilter) == 0 {
						events = append(events, fmt.Sprintf("%s: %s", event.Op, event.Name))
					}
				case err := <-watcher.Errors:
					events = append(events, fmt.Sprintf("ERROR: %v", err))
					break loop
				case <-timer.C:
					break loop
				case <-ctx.Done():
					break loop
				}
			}

			text := "No events detected"
			if len(events) > 0 {
				text = strings.Join(events, "\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{
					"path":       args.Path,
					"event_count": len(events),
					"timeout_sec": timeout.Seconds(),
					"success":    true,
				},
			}, nil
		},
	}
}

func parseEventFilter(filter string) map[fsnotify.Op]bool {
	if filter == "" {
		return nil
	}
	m := make(map[fsnotify.Op]bool)
	for _, name := range strings.Split(filter, ",") {
		switch strings.TrimSpace(name) {
		case "create":
			m[fsnotify.Create] = true
		case "write":
			m[fsnotify.Write] = true
		case "remove":
			m[fsnotify.Remove] = true
		case "rename":
			m[fsnotify.Rename] = true
		case "chmod":
			m[fsnotify.Chmod] = true
		}
	}
	return m
}
