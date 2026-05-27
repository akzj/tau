package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// RaceCheckTool creates a race condition detection tool.
//
// Parameters:
//
//	path    (string, required)  — package path to check for race conditions
//	timeout (number, optional)  — test timeout in seconds (default: 60)
func RaceCheckTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path to check for race conditions"},
			"timeout": {"type": "number", "description": "Test timeout in seconds (default: 60)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "race_check",
		Description: "Race condition detector. Runs go test -race on specified packages, parses race reports into structured output: file+line+goroutine+stack trace.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string  `json:"path"`
				Timeout float64 `json:"timeout"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Timeout <= 0 {
				args.Timeout = 60
			}

			timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(args.Timeout)*time.Second)
			defer cancel()

			// Create temp output file for race log
			tmpDir, err := os.MkdirTemp("", "tau-racecheck-*")
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("cannot create temp dir: %w", err)
			}
			defer os.RemoveAll(tmpDir)

			raceLog := filepath.Join(tmpDir, "race.log")

			cmd := exec.CommandContext(
				timeoutCtx,
				"go", "test", "-race", "-count=1",
				"-timeout="+fmt.Sprintf("%.0fs", args.Timeout),
				args.Path,
			)
			cmd.Dir = WorkspaceRoot
			cmd.Env = append(os.Environ(), "GORACE=log_path="+raceLog)

			output, testErr := cmd.CombinedOutput()
			testOutput := string(output)

			// Parse race reports
			races := parseRaceOutput(testOutput, raceLog)

			return buildRaceResult(races, testOutput, args.Path), testErr
		},
	}
}

type raceEntry struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Goroutine string `json:"goroutine"`
	Operation string `json:"operation"`
	Trace    string `json:"trace"`
}

func parseRaceOutput(testOutput, raceLogPrefix string) []raceEntry {
	// First try the test output (race output goes to stderr which we capture)
	races := parseRaceReport(testOutput)
	if len(races) > 0 {
		return races
	}

	// Try reading race log files
	dir := filepath.Dir(raceLogPrefix)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), filepath.Base(raceLogPrefix)) {
			continue
		}
		if strings.HasSuffix(e.Name(), ".done") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		races = append(races, parseRaceReport(string(data))...)
	}
	return races
}

var (
	raceWarnRe     = regexp.MustCompile(`WARNING: DATA RACE`)
	raceGoroutineRe = regexp.MustCompile(`Goroutine (\d+)`)
	raceFileLineRe   = regexp.MustCompile(`^\s+([^:]+):(\d+)`)
	raceWriteReadRe  = regexp.MustCompile(`(Read|Write) at`)
)

func parseRaceReport(text string) []raceEntry {
	if !raceWarnRe.MatchString(text) {
		return nil
	}

	var races []raceEntry

	// Split by "WARNING: DATA RACE" sections
	sections := raceWarnRe.Split(text, -1)
	for i, section := range sections {
		if i == 0 && !strings.Contains(text, "WARNING: DATA RACE") {
			continue
		}
		entry := parseRaceSection(section)
		if entry.File != "" {
			races = append(races, entry)
		}
	}

	return races
}

func parseRaceSection(section string) raceEntry {
	entry := raceEntry{}
	lines := strings.Split(section, "\n")

	// Extract goroutine info
	for _, line := range lines {
		match := raceGoroutineRe.FindStringSubmatch(line)
		if len(match) >= 2 {
			entry.Goroutine = match[1]
			break
		}
	}

	// Extract operation type
	for _, line := range lines {
		match := raceWriteReadRe.FindStringSubmatch(line)
		if len(match) >= 2 {
			entry.Operation = match[1] + " at ..."
			break
		}
	}

	// Extract file:line from first stack frame
	for _, line := range lines {
		line = strings.TrimRight(line, " ")
		match := raceFileLineRe.FindStringSubmatch(line)
		if len(match) >= 3 {
			entry.File = match[1]
			fmt.Sscanf(match[2], "%d", &entry.Line)
			break
		}
	}

	// Build abbreviated trace (first 10 relevant lines)
	var traceLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		traceLines = append(traceLines, trimmed)
		if len(traceLines) >= 10 {
			break
		}
	}
	entry.Trace = strings.Join(traceLines, "\n")

	return entry
}

func buildRaceResult(races []raceEntry, testOutput string, path string) core.ToolResult {
	if len(races) == 0 {
		// Check if test ran successfully without race
		hasRaceInOutput := strings.Contains(strings.ToLower(testOutput), "data race")
		if hasRaceInOutput {
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: "Race condition detected but could not be parsed. Raw output:\n\n" + truncateStr(testOutput, OutputCap)}},
				Details: map[string]any{"path": path, "success": false, "race_count": 1},
			}
		}
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No race conditions detected."}},
			Details: map[string]any{"path": path, "race_count": 0, "success": true},
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("⚠ %d race condition(s) detected\n", len(races)))

	for i, r := range races {
		lines = append(lines, fmt.Sprintf("--- Race #%d ---", i+1))
		lines = append(lines, fmt.Sprintf("  Goroutine: %s", r.Goroutine))
		lines = append(lines, fmt.Sprintf("  Operation: %s", r.Operation))
		lines = append(lines, fmt.Sprintf("  Location:  %s:%d", r.File, r.Line))
		lines = append(lines, "  Stack trace:")
		for _, tl := range strings.Split(r.Trace, "\n") {
			lines = append(lines, "    "+tl)
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
			"path":       path,
			"race_count": len(races),
			"success":    false,
			"races":      races,
		},
	}
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "\n... (truncated)"
}