package tools

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/akzj/tau/core"
)

// CSVAnalyzeTool profiles a CSV file — column types, null counts, distinct values,
// min/max/mean for numeric columns, and value distributions.
//
// Parameters:
//
//	csv_file    (string, required)  — CSV file to profile
//	sample_size (number, optional)  — max rows to analyze (default: 1000)
func CSVAnalyzeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"csv_file": {"type": "string", "description": "CSV file to profile"},
			"sample_size": {"type": "number", "description": "Maximum rows to analyze (default: 1000)"}
		},
		"required": ["csv_file"]
	}`)

	return core.Tool{
		Name:        "csv_analyze",
		Description: "Profile CSV data: column types, null counts, distinct values, min/max/mean for numeric columns, value distributions. Returns structured profiling report.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				CSVFile    string  `json:"csv_file"`
				SampleSize float64 `json:"sample_size"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.CSVFile == "" {
				return core.ToolResult{}, fmt.Errorf("csv_file required")
			}
			sampleSize := int(args.SampleSize)
			if sampleSize <= 0 {
				sampleSize = 1000
			}

			resolved, err := ResolvePath(args.CSVFile)
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
					Details: map[string]any{"csv_file": args.CSVFile, "rows": 0},
				}, nil
			}

			headers := records[0]
			rows := records[1:]
			totalRows := len(rows)

			// Limit to sample size
			if len(rows) > sampleSize {
				rows = rows[:sampleSize]
			}

			profile := profileCSV(headers, rows, totalRows, sampleSize)

			return buildCSVAnalyzeResult(profile, args.CSVFile, totalRows, len(rows)), nil
		},
	}
}

// csvColumnProfile holds profiling data for one column.
type csvColumnProfile struct {
	Name          string   `json:"name"`
	InferredType  string   `json:"inferred_type"`
	NullCount     int      `json:"null_count"`
	NullPct       float64  `json:"null_pct"`
	DistinctCount int      `json:"distinct_count"`
	MinVal        *float64 `json:"min_val,omitempty"`
	MaxVal        *float64 `json:"max_val,omitempty"`
	MeanVal       *float64 `json:"mean_val,omitempty"`
	TopValues     []string `json:"top_values,omitempty"`
}

func profileCSV(headers []string, rows [][]string, totalRows int, sampleSize int) []csvColumnProfile {
	numCols := len(headers)
	profiles := make([]csvColumnProfile, numCols)

	// Per-column accumulators
	nullCounts := make([]int, numCols)
	valueSlices := make([][]string, numCols)
	for i := range valueSlices {
		valueSlices[i] = make([]string, 0, len(rows))
	}

	for _, row := range rows {
		for colIdx := 0; colIdx < numCols && colIdx < len(row); colIdx++ {
			val := strings.TrimSpace(row[colIdx])
			if val == "" {
				nullCounts[colIdx]++
				valueSlices[colIdx] = append(valueSlices[colIdx], "")
			} else {
				valueSlices[colIdx] = append(valueSlices[colIdx], val)
			}
		}
		// Pad missing columns with null
		for colIdx := len(row); colIdx < numCols; colIdx++ {
			nullCounts[colIdx]++
			valueSlices[colIdx] = append(valueSlices[colIdx], "")
		}
	}

	// Analyze each column
	for colIdx := 0; colIdx < numCols; colIdx++ {
		nonNull := make([]string, 0, len(valueSlices[colIdx]))
		for _, v := range valueSlices[colIdx] {
			if v != "" {
				nonNull = append(nonNull, v)
			}
		}

		p := csvColumnProfile{
			Name:         headers[colIdx],
			NullCount:    nullCounts[colIdx],
			InferredType: inferColumnType(nonNull),
		}
		if len(valueSlices[colIdx]) > 0 {
			p.NullPct = float64(nullCounts[colIdx]) / float64(len(valueSlices[colIdx])) * 100
		}

		// Distinct count
		distinct := make(map[string]bool)
		for _, v := range nonNull {
			distinct[v] = true
		}
		p.DistinctCount = len(distinct)

		// Numeric statistics
		if p.InferredType == "number" {
			var nums []float64
			for _, v := range nonNull {
				if n, err := strconv.ParseFloat(v, 64); err == nil {
					nums = append(nums, n)
				}
			}
			if len(nums) > 0 {
				nMin := nums[0]
				nMax := nums[0]
				sum := 0.0
				for _, n := range nums {
					if n < nMin {
						nMin = n
					}
					if n > nMax {
						nMax = n
					}
					sum += n
				}
				p.MinVal = &nMin
				p.MaxVal = &nMax
				mean := sum / float64(len(nums))
				p.MeanVal = &mean
			}
		}

		// Top values (for low-cardinality columns)
		if p.DistinctCount <= 20 && p.DistinctCount > 0 {
			freq := make(map[string]int)
			for _, v := range nonNull {
				freq[v]++
			}
			type pair struct {
				val   string
				count int
			}
			var pairs []pair
			for v, c := range freq {
				pairs = append(pairs, pair{v, c})
			}
			sort.Slice(pairs, func(i, j int) bool { return pairs[i].count > pairs[j].count })
			topN := 5
			if len(pairs) < topN {
				topN = len(pairs)
			}
			for i := 0; i < topN; i++ {
				p.TopValues = append(p.TopValues, fmt.Sprintf("%s (%d)", pairs[i].val, pairs[i].count))
			}
		}

		profiles[colIdx] = p
	}

	return profiles
}

// inferColumnType determines the likely data type of a column.
func inferColumnType(values []string) string {
	if len(values) == 0 {
		return "empty"
	}

	// Check if all non-empty values are numeric
	allNumeric := true
	hasNonNumeric := false
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			allNumeric = false
			hasNonNumeric = true
			break
		}
	}

	if allNumeric && len(values) > 0 {
		return "number"
	}

	// Check for boolean
	allBool := true
	for _, v := range values {
		if v == "" {
			continue
		}
		lower := strings.ToLower(v)
		if lower != "true" && lower != "false" && lower != "1" && lower != "0" && lower != "yes" && lower != "no" {
			allBool = false
			break
		}
	}
	if allBool && len(values) > 0 {
		return "boolean"
	}

	// Check for date patterns (simple heuristic)
	dateCount := 0
	for _, v := range values {
		if v == "" {
			continue
		}
		if isDateLike(v) {
			dateCount++
		}
	}
	if dateCount > len(values)/2 && len(values) > 0 {
		return "date"
	}

	_ = hasNonNumeric
	return "string"
}

// isDateLike checks if a string looks like a date.
func isDateLike(s string) bool {
	// Match YYYY-MM-DD, MM/DD/YYYY, etc.
	if len(s) >= 8 && len(s) <= 10 {
		if strings.Contains(s, "-") || strings.Contains(s, "/") {
			parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '/' })
			if len(parts) == 3 {
				for _, p := range parts {
					if _, err := strconv.Atoi(p); err != nil {
						return false
					}
				}
				return true
			}
		}
	}
	return false
}

func buildCSVAnalyzeResult(profiles []csvColumnProfile, file string, totalRows int, sampled int) core.ToolResult {
	var lines []string
	lines = append(lines, fmt.Sprintf("## CSV Profile: %s", file))
	lines = append(lines, fmt.Sprintf("Total rows: %d (sampled: %d)\n", totalRows, sampled))

	for _, p := range profiles {
		lines = append(lines, fmt.Sprintf("### %s (%s)", p.Name, p.InferredType))
		lines = append(lines, fmt.Sprintf("  Null: %d (%.1f%%)", p.NullCount, p.NullPct))
		lines = append(lines, fmt.Sprintf("  Distinct: %d", p.DistinctCount))

		if p.InferredType == "number" {
			if p.MinVal != nil {
				lines = append(lines, fmt.Sprintf("  Min: %v", roundFloat(*p.MinVal)))
			}
			if p.MaxVal != nil {
				lines = append(lines, fmt.Sprintf("  Max: %v", roundFloat(*p.MaxVal)))
			}
			if p.MeanVal != nil {
				lines = append(lines, fmt.Sprintf("  Mean: %v", roundFloat(*p.MeanVal)))
			}
		}

		if len(p.TopValues) > 0 {
			lines = append(lines, fmt.Sprintf("  Top: %s", strings.Join(p.TopValues, ", ")))
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
			"csv_file":   file,
			"total_rows": totalRows,
			"sampled":    sampled,
			"columns":    len(profiles),
			"profiles":   profiles,
			"success":    true,
		},
	}
}

func roundFloat(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return f
	}
	return math.Round(f*100) / 100
}
