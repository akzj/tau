package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// JSONQueryTool creates a jq-style JSON query tool.
//
// Parameters:
//
//	file   (string, required) — JSON file path
//	query  (string, optional) — dot-path notation (e.g., "users.0.name")
//	action (string, optional, default: extract) — extract | list | filter
//
// Uses encoding/json for traversal. Supports dot-path access into nested objects and arrays.
func JSONQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "JSON file path"},
			"query": {"type": "string", "description": "Dot-path query (e.g., 'users.0.name', 'items.*.id')"},
			"action": {"type": "string", "description": "Action: extract (default), list, filter"}
		},
		"required": ["file"]
	}`)

	return core.Tool{
		Name:        "json_query",
		Description: "Query JSON files with dot-path notation. Supports extract, list, filter actions.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File   string `json:"file"`
				Query  string `json:"query"`
				Action string `json:"action"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}
			if args.Action == "" {
				args.Action = "extract"
			}

			resolved, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			data, err := os.ReadFile(resolved)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file: %w", err)
			}

			var root any
			if err := json.Unmarshal(data, &root); err != nil {
				return core.ToolResult{}, fmt.Errorf("parse JSON: %w", err)
			}

			switch args.Action {
			case "list":
				return jsonList(root, args.File, args.Query)
			case "filter":
				return jsonFilter(root, args.File, args.Query)
			default: // extract
				return jsonExtract(root, args.File, args.Query)
			}
		},
	}
}

func jsonExtract(root any, file, query string) (core.ToolResult, error) {
	if query == "" {
		// Pretty print entire JSON
		pretty, _ := json.MarshalIndent(root, "", "  ")
		text := string(pretty)
		if len(text) > 4000 {
			text = text[:4000] + "\n... (truncated)"
		}
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: text}},
			Details: map[string]any{"file": file},
		}, nil
	}

	val := resolveJSONPath(root, query)
	if val == nil {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("(path not found: %s)", query)}},
			Details: map[string]any{"file": file, "query": query, "found": false},
		}, nil
	}

	pretty, _ := json.MarshalIndent(val, "", "  ")
	text := string(pretty)
	if len(text) > 4000 {
		text = text[:4000] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("## JSON Extract: %s\n%s\n\n%s", file, query, text)}},
		Details: map[string]any{"file": file, "query": query, "found": true},
	}, nil
}

func jsonList(root any, file, query string) (core.ToolResult, error) {
	target := root
	if query != "" {
		target = resolveJSONPath(root, query)
	}

	switch v := target.(type) {
	case []any:
		var lines []string
		for i, item := range v {
			switch iv := item.(type) {
			case map[string]any:
				// Try to show key fields
				keys := make([]string, 0, len(iv))
				for k := range iv {
					keys = append(keys, k)
				}
				lines = append(lines, fmt.Sprintf("%d. {%d keys: %s}", i, len(iv), strings.Join(keys[:min(5, len(keys))], ", ")))
			default:
				pretty, _ := json.Marshal(iv)
				lines = append(lines, fmt.Sprintf("%d. %s", i, string(pretty)))
			}
		}
		output := fmt.Sprintf("## JSON List: %s (%d items)\n\n", query, len(v))
		for _, l := range lines {
			output += l + "\n"
		}
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: output}},
			Details: map[string]any{"file": file, "query": query, "count": len(v)},
		}, nil
	case map[string]any:
		var lines []string
		for k := range v {
			lines = append(lines, k)
		}
		output := fmt.Sprintf("## JSON Keys: %s (%d keys)\n\n", query, len(v))
		for _, k := range lines {
			output += k + "\n"
		}
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: output}},
			Details: map[string]any{"file": file, "query": query, "count": len(v)},
		}, nil
	default:
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("(not a list or object at path: %s)", query)}},
		}, nil
	}
}

func jsonFilter(root any, file, query string) (core.ToolResult, error) {
	// filter is list with the query being the path to a list
	// returns each item as a line
	target := root
	if query != "" {
		target = resolveJSONPath(root, query)
	}

	arr, ok := target.([]any)
	if !ok {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("(not an array at path: %s)", query)}},
		}, nil
	}

	var lines []string
	for _, item := range arr {
		pretty, _ := json.Marshal(item)
		s := string(pretty)
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		lines = append(lines, s)
	}

	output := fmt.Sprintf("## JSON Filter: %s (%d items)\n\n", query, len(arr))
	for _, l := range lines {
		output += l + "\n"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{"file": file, "query": query, "count": len(arr)},
	}, nil
}

func resolveJSONPath(root any, path string) any {
	if path == "" || path == "." {
		return root
	}

	parts := strings.Split(path, ".")
	current := root
	for _, part := range parts {
		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil
			}
		case []any:
			// Try numeric index
			idx := 0
			fmt.Sscanf(part, "%d", &idx)
			if idx < 0 || idx >= len(v) {
				return nil
			}
			current = v[idx]
		default:
			return nil
		}
	}
	return current
}


