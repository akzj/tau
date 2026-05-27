package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/akzj/tau/core"
)

// CSVQueryTool creates a CSV filter + stats tool.
//
// Parameters:
//
//	file   (string, required) — CSV file path
//	action (string, required) — count | sum | avg | filter | columns
//	column (string, optional) — column name for sum/avg/filter
//	value  (string, optional) — value to filter by
//
// Uses encoding/csv for parsing. First row is treated as header.
func CSVQueryTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "CSV file path"},
			"action": {"type": "string", "description": "Action: count, sum, avg, filter, columns"},
			"column": {"type": "string", "description": "Column name for sum/avg/filter"},
			"value": {"type": "string", "description": "Value to filter by (for filter action)"}
		},
		"required": ["file", "action"]
	}`)

	return core.Tool{
		Name:        "csv_query",
		Description: "Query CSV files: count, sum, avg, filter rows, list columns. Uses encoding/csv.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File   string `json:"file"`
				Action string `json:"action"`
				Column string `json:"column"`
				Value  string `json:"value"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}

			resolved, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			f, err := os.Open(resolved)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			reader := csv.NewReader(f)
			records, err := reader.ReadAll()
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("parse CSV: %w", err)
			}

			if len(records) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "(empty CSV file)"}},
				}, nil
			}

			headers := records[0]
			rows := records[1:]

			// Build header index
			headerIdx := make(map[string]int)
			for i, h := range headers {
				headerIdx[strings.TrimSpace(h)] = i
			}

			switch args.Action {
			case "columns":
				output := "## CSV Columns\n\n"
				for i, h := range headers {
					output += fmt.Sprintf("%d. %s\n", i+1, h)
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"file": args.File, "columns": len(headers), "rows": len(rows)},
				}, nil

			case "count":
				count := len(rows)
				output := fmt.Sprintf("## CSV Count\n\n%d rows", count)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"file": args.File, "count": count},
				}, nil

			case "sum":
				if args.Column == "" {
					return core.ToolResult{}, fmt.Errorf("column required for sum")
				}
				colIdx, ok := headerIdx[args.Column]
				if !ok {
					return core.ToolResult{}, fmt.Errorf("column not found: %s", args.Column)
				}
				sum := 0.0
				count := 0
				for _, row := range rows {
					if colIdx < len(row) {
						v, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx]), 64)
						if err == nil {
							sum += v
							count++
						}
					}
				}
				output := fmt.Sprintf("## CSV Sum: %s\n\nSum: %.2f (from %d numeric values)", args.Column, sum, count)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"column": args.Column, "sum": sum, "count": count},
				}, nil

			case "avg":
				if args.Column == "" {
					return core.ToolResult{}, fmt.Errorf("column required for avg")
				}
				colIdx, ok := headerIdx[args.Column]
				if !ok {
					return core.ToolResult{}, fmt.Errorf("column not found: %s", args.Column)
				}
				sum := 0.0
				count := 0
				for _, row := range rows {
					if colIdx < len(row) {
						v, err := strconv.ParseFloat(strings.TrimSpace(row[colIdx]), 64)
						if err == nil {
							sum += v
							count++
						}
					}
				}
				avg := 0.0
				if count > 0 {
					avg = sum / float64(count)
				}
				output := fmt.Sprintf("## CSV Average: %s\n\nAverage: %.2f (from %d values)", args.Column, avg, count)
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"column": args.Column, "avg": avg, "count": count},
				}, nil

			case "filter":
				if args.Column == "" {
					return core.ToolResult{}, fmt.Errorf("column required for filter")
				}
				colIdx, ok := headerIdx[args.Column]
				if !ok {
					return core.ToolResult{}, fmt.Errorf("column not found: %s", args.Column)
				}
				var matched [][]string
				for _, row := range rows {
					if colIdx < len(row) && strings.TrimSpace(row[colIdx]) == args.Value {
						matched = append(matched, row)
					}
				}
				output := fmt.Sprintf("## CSV Filter: %s = %s\n\nMatched: %d rows\n\n", args.Column, args.Value, len(matched))
				for i, row := range matched {
					if i >= 20 {
						output += fmt.Sprintf("... (%d more rows)", len(matched)-20)
						break
					}
					output += strings.Join(row, " | ") + "\n"
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: output}},
					Details: map[string]any{"column": args.Column, "value": args.Value, "matched": len(matched)},
				}, nil

			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use count/sum/avg/filter/columns)", args.Action)
			}
		},
	}
}
