package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// TestGapTool creates a test coverage gap analyzer.
//
// Parameters:
//
//	path      (string, required)  — package path to analyze (e.g., ./pkg/...)
//	threshold (number, optional)  — minimum acceptable coverage percentage (default: 80)
func TestGapTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path to analyze coverage gaps"},
			"threshold": {"type": "number", "description": "Minimum acceptable coverage percentage (default: 80)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "test_gap",
		Description: "Test coverage gap analyzer. Runs go test -coverprofile, parses coverage profiles, identifies files with low coverage and uncovered lines. Returns file+uncovered_lines+coverage_pct.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path      string  `json:"path"`
				Threshold float64 `json:"threshold"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Threshold <= 0 {
				args.Threshold = 80
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
			defer cancel()

			// Create temp coverage profile
			tmpDir, err := os.MkdirTemp("", "tau-testgap-*")
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("cannot create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			coverProfile := filepath.Join(tmpDir, "coverage.out")

			// Run go test with coverage
			cmd := exec.CommandContext(timeoutCtx, "go", "test", "-count=1", "-coverprofile="+coverProfile, args.Path)
			cmd.Dir = WorkspaceRoot
			testOutput, testErr := cmd.CombinedOutput()

			// Try without -count=1 if first attempt fails
			if testErr != nil {
				cmd2 := exec.CommandContext(timeoutCtx, "go", "test", "-coverprofile="+coverProfile, args.Path)
				cmd2.Dir = WorkspaceRoot
				testOutput, testErr = cmd2.CombinedOutput()
			}

			_ = testOutput

			// Parse coverage profile
			gaps, err := parseCoverageProfile(coverProfile, args.Threshold)
			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Coverage analysis: %v\nTest output:\n%s", err, string(testOutput))}},
					Details: map[string]any{"path": args.Path, "success": false},
				}, nil
			}

			return buildTestGapResult(gaps, args.Path, args.Threshold), nil
		},
	}
}

type testGapEntry struct {
	File             string  `json:"file"`
	CoveragePct      float64 `json:"coverage_pct"`
	UncoveredLines   []int   `json:"uncovered_lines,omitempty"`
	BelowThreshold   bool    `json:"below_threshold"`
}

func parseCoverageProfile(profilePath string, threshold float64) ([]testGapEntry, error) {
	data, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("no coverage data: %w", err)
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("empty coverage profile")
	}

	// Parse go cover profile
	// Format: mode: set
	// file:startLine.startCol,endLine.endCol numStatements count
	content := string(data)
	fileBlocks := make(map[string][]coverageBlock)

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		file := parts[0]
		rest := parts[1]

		// Parse: startLine.startCol,endLine.endCol numStatements count
		fields := strings.Fields(rest)
		if len(fields) < 2 {
			continue
		}
		loc := fields[0]
		numStmts, _ := strconv.Atoi(fields[1])
		count := 0
		if len(fields) >= 3 {
			count, _ = strconv.Atoi(fields[2])
		}

		// Parse startLine.startCol,endLine.endCol
		locParts := strings.Split(loc, ",")
		if len(locParts) < 2 {
			continue
		}
		startParts := strings.Split(locParts[0], ".")
		endParts := strings.Split(locParts[1], ".")
		if len(startParts) < 1 || len(endParts) < 1 {
			continue
		}
		startLine, _ := strconv.Atoi(startParts[0])
		endLine, _ := strconv.Atoi(endParts[0])

		fileBlocks[file] = append(fileBlocks[file], coverageBlock{
			startLine: startLine,
			endLine:   endLine,
			stmts:     numStmts,
			count:     count,
		})
	}

	// Calculate per-file coverage
	var gaps []testGapEntry
	for file, blocks := range fileBlocks {
		totalStmts := 0
		coveredStmts := 0
		var uncoveredLines []int

		for _, b := range blocks {
			totalStmts += b.stmts
			if b.count > 0 {
				coveredStmts += b.stmts
			} else {
				for l := b.startLine; l <= b.endLine; l++ {
					uncoveredLines = append(uncoveredLines, l)
				}
			}
		}

		pct := 0.0
		if totalStmts > 0 {
			pct = float64(coveredStmts) / float64(totalStmts) * 100
		}

		gaps = append(gaps, testGapEntry{
			File:           file,
			CoveragePct:    pct,
			UncoveredLines: uncoveredLines,
			BelowThreshold: pct < threshold,
		})
	}

	return gaps, nil
}

type coverageBlock struct {
	startLine int
	endLine   int
	stmts     int
	count     int
}

func buildTestGapResult(gaps []testGapEntry, pathArg string, threshold float64) core.ToolResult {
	if len(gaps) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No test coverage data found."}},
			Details: map[string]any{"path": pathArg, "files_analyzed": 0, "success": true},
		}
	}

	var belowThreshold []testGapEntry
	for _, g := range gaps {
		if g.BelowThreshold {
			belowThreshold = append(belowThreshold, g)
		}
	}

	var lines []string
	for _, g := range gaps {
		marker := "✓"
		if g.BelowThreshold {
			marker = "✗"
		}
		uncoveredStr := ""
		if len(g.UncoveredLines) > 0 && len(g.UncoveredLines) <= 10 {
			var strs []string
			for _, l := range g.UncoveredLines {
				strs = append(strs, strconv.Itoa(l))
			}
			uncoveredStr = "  lines=[" + strings.Join(strs, ",") + "]"
		} else if len(g.UncoveredLines) > 10 {
			uncoveredStr = fmt.Sprintf("  lines=[%d uncovered lines]", len(g.UncoveredLines))
		}
		lines = append(lines, fmt.Sprintf("%s %s\t%.1f%%%s", marker, g.File, g.CoveragePct, uncoveredStr))
	}

	output := fmt.Sprintf("Test coverage gap analysis (threshold: %.0f%%)\n", threshold)
	output += fmt.Sprintf("Files below threshold: %d/%d\n\n", len(belowThreshold), len(gaps))
	output += strings.Join(lines, "\n")

	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"path":              pathArg,
			"threshold":         threshold,
			"files_analyzed":    len(gaps),
			"below_threshold":   len(belowThreshold),
			"success":           len(belowThreshold) == 0,
			"entries":           gaps,
		},
	}
}