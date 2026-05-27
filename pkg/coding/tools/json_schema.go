package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// JSONSchemaTool generates a JSON Schema (Draft-07) from a JSON file.
//
// Parameters:
//
//	json_file           (string, required) — JSON file to analyze
//	output_schema_path  (string, optional) — path to write the schema; if empty, returned in result
func JSONSchemaTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"json_file": {"type": "string", "description": "JSON file to generate schema from"},
			"output_schema_path": {"type": "string", "description": "Optional path to write the generated schema"}
		},
		"required": ["json_file"]
	}`)

	return core.Tool{
		Name:        "json_schema",
		Description: "Generate JSON Schema (Draft-07) from a JSON file. Infers types, handles nested objects and arrays, detects nulls.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				JSONFile         string `json:"json_file"`
				OutputSchemaPath string `json:"output_schema_path"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.JSONFile == "" {
				return core.ToolResult{}, fmt.Errorf("json_file required")
			}

			resolved, err := ResolvePath(args.JSONFile)
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

			schemaObj := inferSchema(root)
			schemaObj["$schema"] = "http://json-schema.org/draft-07/schema#"
			schemaObj["title"] = strings.TrimSuffix(args.JSONFile, ".json")

			schemaBytes, err := json.MarshalIndent(schemaObj, "", "  ")
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("marshal schema: %w", err)
			}

			// Write to file if output path specified
			if args.OutputSchemaPath != "" {
				resolvedOut, err := ResolvePath(args.OutputSchemaPath)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("resolve output path: %w", err)
				}
				if err := os.WriteFile(resolvedOut, schemaBytes, 0644); err != nil {
					return core.ToolResult{}, fmt.Errorf("write schema: %w", err)
				}
			}

			text := string(schemaBytes)
			if len(text) > OutputCap {
				text = text[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{
					"json_file":        args.JSONFile,
					"output_schema_path": args.OutputSchemaPath,
					"success":          true,
				},
			}, nil
		},
	}
}

// inferSchema walks a JSON value and returns a JSON Schema object.
func inferSchema(v any) map[string]any {
	return inferSchemaRecursive(v, 0)
}

func inferSchemaRecursive(v any, depth int) map[string]any {
	if depth > 20 {
		return map[string]any{"type": "string", "description": "max depth exceeded"}
	}

	if v == nil {
		return map[string]any{"type": "null"}
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice:
		return inferArraySchema(v.([]any), depth)
	case reflect.Map:
		return inferObjectSchema(v.(map[string]any), depth)
	default:
		return map[string]any{"type": "string"}
	}
}

func inferArraySchema(arr []any, depth int) map[string]any {
	if len(arr) == 0 {
		return map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	}

	// Collect types of all elements
	var itemSchemas []map[string]any
	for i, item := range arr {
		if i >= 100 {
			break
		}
		itemSchemas = append(itemSchemas, inferSchemaRecursive(item, depth+1))
	}

	// Merge: if all same type, use that; otherwise use oneOf
	merged := mergeSchemas(itemSchemas)
	s := map[string]any{"type": "array", "items": merged}
	if len(arr) > 100 {
		s["description"] = fmt.Sprintf("sampled %d of %d items", 100, len(arr))
	}
	return s
}

func inferObjectSchema(obj map[string]any, depth int) map[string]any {
	props := make(map[string]any)
	required := []string{}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		props[k] = inferSchemaRecursive(obj[k], depth+1)
		required = append(required, k)
	}

	return map[string]any{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// mergeSchemas merges multiple schemas. If all are the same type, return that type.
// Otherwise return a oneOf.
func mergeSchemas(schemas []map[string]any) map[string]any {
	if len(schemas) == 0 {
		return map[string]any{"type": "string"}
	}
	if len(schemas) == 1 {
		return schemas[0]
	}

	// Check if all schemas are identical
	allSame := true
	for i := 1; i < len(schemas); i++ {
		if !schemasEqual(schemas[0], schemas[i]) {
			allSame = false
			break
		}
	}
	if allSame {
		return schemas[0]
	}

	// Collect unique types
	seen := make(map[string]bool)
	var unique []map[string]any
	for _, s := range schemas {
		key := schemaKey(s)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, s)
		}
	}

	if len(unique) == 1 {
		return unique[0]
	}

	// If we have type differences, use oneOf with nullable support
	return map[string]any{
		"oneOf": unique,
	}
}

func schemasEqual(a, b map[string]any) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}

func schemaKey(s map[string]any) string {
	j, _ := json.Marshal(s)
	return string(j)
}
