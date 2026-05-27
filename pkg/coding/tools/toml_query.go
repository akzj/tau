package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/BurntSushi/toml"
	"github.com/akzj/tau/core"
)

// TOMLQueryTool creates a TOML query tool.
//
// Parameters:
//
//	file  (string, required) — TOML file path
//	query (string, optional) — dot-separated path (e.g., "database.port")
func TOMLQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "TOML file path"},
			"query": {"type": "string", "description": "Dot-separated path (e.g., database.port). Empty = return entire doc."}
		},
		"required": ["file"]
	}`)

	return core.Tool{
		Name:        "toml_query",
		Description: "Parse and query TOML files using dot-notation paths.",
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

			var obj map[string]any
			if _, err := toml.DecodeFile(args.File, &obj); err != nil {
				return core.ToolResult{}, fmt.Errorf("parse toml: %w", err)
			}

			var result any = obj
			if args.Query != "" {
				result = navigateJSON(obj, args.Query)
			}

			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: string(resultJSON)}},
				Details: map[string]any{"file": args.File, "query": args.Query, "success": true},
			}, nil
		},
	}
}
