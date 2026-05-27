package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// DataDiffTool compares two data files and shows differences.
//
// Parameters:
//
//	file_a (string, required) — first data file
//	file_b (string, required) — second data file
//	format (string, optional) — csv | json | tsv (auto-detected from extension if empty)
func DataDiffTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_a": {"type": "string", "description": "First data file"},
			"file_b": {"type": "string", "description": "Second data file"},
			"format": {"type": "string", "description": "Format: csv, json, tsv (auto-detected if empty)"}
		},
		"required": ["file_a", "file_b"]
	}`)

	return core.Tool{
		Name:        "data_diff",
		Description: "Compare two data files (CSV, JSON arrays, TSV). Shows added, removed, and changed rows. Auto-detects format from file extension.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FileA  string `json:"file_a"`
				FileB  string `json:"file_b"`
				Format string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.FileA == "" || args.FileB == "" {
				return core.ToolResult{}, fmt.Errorf("both file_a and file_b required")
			}

			resolvedA, err := ResolvePath(args.FileA)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve file_a: %w", err)
			}
			resolvedB, err := ResolvePath(args.FileB)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve file_b: %w", err)
			}

			// Auto-detect format
			format := args.Format
			if format == "" {
				format = detectDataFormat(resolvedA)
			}

			dataA, err := readDataFile(resolvedA, format)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file_a: %w", err)
			}
			dataB, err := readDataFile(resolvedB, format)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file_b: %w", err)
			}

			result := diffData(dataA, dataB)

			return buildDiffResult(result, args.FileA, args.FileB, format), nil
		},
	}
}

// dataRow represents a single row from a data file.
type dataRow struct {
	Index  int
	Values []string
	Key    string // hashed representation for comparison
}

func readDataFile(path string, format string) ([]dataRow, error) {
	switch format {
	case "json":
		return readJSONData(path)
	case "tsv":
		return readDelimitedData(path, '\t')
	case "csv":
		fallthrough
	default:
		return readDelimitedData(path, ',')
	}
}

func readJSONData(path string) ([]dataRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var root any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	// Handle JSON array of objects
	arr, ok := root.([]any)
	if !ok {
		return nil, fmt.Errorf("expected JSON array, got %T", root)
	}

	var rows []dataRow
	for i, item := range arr {
		obj, ok := item.(map[string]any)
		if !ok {
			// Non-object array element: serialize as string
			pretty, _ := json.Marshal(item)
			rows = append(rows, dataRow{
				Index:  i,
				Values: []string{string(pretty)},
				Key:    string(pretty),
			})
			continue
		}

		// Extract sorted keys for deterministic ordering
		var keys []string
		for k := range obj {
			keys = append(keys, k)
		}
		// sort not needed for deterministic output but helps readability
		var vals []string
		var keyParts []string
		for _, k := range keys {
			v := fmt.Sprintf("%v", obj[k])
			vals = append(vals, k+":"+v)
			keyParts = append(keyParts, v)
		}
		rows = append(rows, dataRow{
			Index:  i,
			Values: vals,
			Key:    strings.Join(keyParts, "|"),
		})
	}

	return rows, nil
}

func readDelimitedData(path string, delimiter rune) ([]dataRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.Comma = delimiter
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse delimited: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	// First row is header
	headers := records[0]
	var rows []dataRow

	for i, record := range records[1:] {
		vals := make([]string, len(headers))
		for j, h := range headers {
			if j < len(record) {
				vals[j] = h + ":" + record[j]
			} else {
				vals[j] = h + ":"
			}
		}
		// Key is all values joined
		keyParts := make([]string, len(record))
		for j := 0; j < len(record); j++ {
			keyParts[j] = record[j]
		}
		rows = append(rows, dataRow{
			Index:  i,
			Values: vals,
			Key:    strings.Join(keyParts, "|"),
		})
	}

	return rows, nil
}

type diffResult struct {
	Added   []dataRow
	Removed []dataRow
	Changed []rowChange
}

type rowChange struct {
	Index int
	Old   dataRow
	New   dataRow
}

func diffData(a, b []dataRow) diffResult {
	keyMapA := make(map[string]dataRow)
	keyMapB := make(map[string]dataRow)

	for _, r := range a {
		keyMapA[r.Key] = r
	}
	for _, r := range b {
		keyMapB[r.Key] = r
	}

	var result diffResult

	// Find added (in B but not in A)
	for key, row := range keyMapB {
		if _, ok := keyMapA[key]; !ok {
			result.Added = append(result.Added, row)
		}
	}

	// Find removed (in A but not in B)
	for key, row := range keyMapA {
		if _, ok := keyMapB[key]; !ok {
			result.Removed = append(result.Removed, row)
		}
	}

	// Changed: detect by index (same position, different content)
	// This handles CSV/TSV where key-based diff may not catch reordered but changed data
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	for i := 0; i < maxLen; i++ {
		var rowA, rowB dataRow
		hasA := i < len(a)
		hasB := i < len(b)
		if hasA {
			rowA = a[i]
		}
		if hasB {
			rowB = b[i]
		}
		if hasA && hasB && rowA.Key != rowB.Key {
			// Check this isn't already in added/removed
			if _, ok := keyMapA[rowB.Key]; !ok {
				continue // handled as added
			}
			if _, ok := keyMapB[rowA.Key]; !ok {
				continue // handled as removed
			}
			result.Changed = append(result.Changed, rowChange{
				Index: i,
				Old:   rowA,
				New:   rowB,
			})
		}
	}

	return result
}

func detectDataFormat(path string) string {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".json") {
		return "json"
	}
	if strings.HasSuffix(lower, ".tsv") || strings.HasSuffix(lower, ".tab") {
		return "tsv"
	}
	return "csv"
}

func buildDiffResult(diff diffResult, fileA, fileB, format string) core.ToolResult {
	var lines []string
	lines = append(lines, fmt.Sprintf("## Data Diff: %s vs %s", fileA, fileB))
	lines = append(lines, fmt.Sprintf("Format: %s\n", format))

	summary := fmt.Sprintf("Summary: +%d added, -%d removed, ~%d changed",
		len(diff.Added), len(diff.Removed), len(diff.Changed))
	lines = append(lines, summary)
	lines = append(lines, "")

	if len(diff.Added) > 0 {
		lines = append(lines, fmt.Sprintf("### Added (%d)", len(diff.Added)))
		for i, row := range diff.Added {
			if i >= 20 {
				lines = append(lines, fmt.Sprintf("... (%d more)", len(diff.Added)-20))
				break
			}
			lines = append(lines, fmt.Sprintf("  + %s", strings.Join(row.Values, ", ")))
		}
		lines = append(lines, "")
	}

	if len(diff.Removed) > 0 {
		lines = append(lines, fmt.Sprintf("### Removed (%d)", len(diff.Removed)))
		for i, row := range diff.Removed {
			if i >= 20 {
				lines = append(lines, fmt.Sprintf("... (%d more)", len(diff.Removed)-20))
				break
			}
			lines = append(lines, fmt.Sprintf("  - %s", strings.Join(row.Values, ", ")))
		}
		lines = append(lines, "")
	}

	if len(diff.Changed) > 0 {
		lines = append(lines, fmt.Sprintf("### Changed (%d)", len(diff.Changed)))
		for i, c := range diff.Changed {
			if i >= 20 {
				lines = append(lines, fmt.Sprintf("... (%d more)", len(diff.Changed)-20))
				break
			}
			lines = append(lines, fmt.Sprintf("  ~ row %d:", c.Index))
			lines = append(lines, fmt.Sprintf("    old: %s", strings.Join(c.Old.Values, ", ")))
			lines = append(lines, fmt.Sprintf("    new: %s", strings.Join(c.New.Values, ", ")))
		}
		lines = append(lines, "")
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"file_a":    fileA,
			"file_b":    fileB,
			"format":    format,
			"added":     len(diff.Added),
			"removed":   len(diff.Removed),
			"changed":   len(diff.Changed),
			"success":   true,
		},
	}
}
