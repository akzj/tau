package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
	"gopkg.in/yaml.v3"
)

// YAMLQueryTool creates a YAML query tool.
//
// Parameters:
//
//	file  (string, required) — YAML file path
//	query (string, optional) — dot-separated path (e.g., "servers.0.host"), empty = return entire doc
func YAMLQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "YAML file path"},
			"query": {"type": "string", "description": "Dot-separated path (e.g., servers.0.host). Empty = return entire doc."}
		},
		"required": ["file"]
	}`)

	return core.Tool{
		Name:        "yaml_query",
		Description: "Parse and query YAML files using dot-notation paths. Supports nested objects and array indexing.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File  string `json:"file"`
				Query string `json:"query"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}

			data, err := os.ReadFile(args.File)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file: %w", err)
			}

			var obj any
			if err := yaml.Unmarshal(data, &obj); err != nil {
				return core.ToolResult{}, fmt.Errorf("parse yaml: %w", err)
			}

			// Normalize to []any / map[string]any for consistent navigation
			result := normalizeYAML(obj)
			if args.Query != "" {
				result = navigateJSON(result, args.Query)
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: string(resultJSON)}},
				Details: map[string]any{"file": args.File, "query": args.Query, "success": true},
			}, nil
		},
	}
}

// normalizeYAML converts yaml.v3 node trees to plain maps/slices.
func normalizeYAML(obj any) any {
	switch v := obj.(type) {
	case map[string]any:
		return v
	case []any:
		return v
	case yaml.Node:
		return nodeToAny(&v)
	case *yaml.Node:
		return nodeToAny(v)
	default:
		return obj
	}
}

func nodeToAny(n *yaml.Node) any {
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) > 0 {
			return nodeToAny(n.Content[0])
		}
		return nil
	case yaml.MappingNode:
		m := make(map[string]any)
		for i := 0; i+1 < len(n.Content); i += 2 {
			m[n.Content[i].Value] = nodeToAny(n.Content[i+1])
		}
		return m
	case yaml.SequenceNode:
		var s []any
		for _, child := range n.Content {
			s = append(s, nodeToAny(child))
		}
		return s
	case yaml.ScalarNode:
		return n.Value
	default:
		return n.Value
	}
}

// navigateJSON traverses a nested map/slice using a dot-separated path.
func navigateJSON(obj any, path string) any {
	if path == "" {
		return obj
	}
	parts := strings.Split(path, ".")
	current := obj
	for _, p := range parts {
		switch v := current.(type) {
		case map[string]any:
			var ok bool
			current, ok = v[p]
			if !ok {
				return nil
			}
		case []any:
			idx := 0
			fmt.Sscanf(p, "%d", &idx)
			if idx >= 0 && idx < len(v) {
				current = v[idx]
			} else {
				return nil
			}
		default:
			return nil
		}
	}
	return current
}
