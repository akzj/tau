package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// LintCheckTool creates a multi-linter runner tool.
//
// Parameters:
//
//	path    (string, required) — file or directory path to lint
//	linters (string, optional) — comma-separated linters: gofmt, goimports, govet, staticcheck, golangci-lint
//	                              (default: govet)
func LintCheckTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path to lint"},
			"linters": {"type": "string", "description": "Comma-separated linters: gofmt,goimports,govet,staticcheck,golangci-lint"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "lint_check",
		Description: "Multi-linter runner. Runs configured linters (gofmt, goimports, govet, staticcheck, golangci-lint) and aggregates results with file:line+severity+message+linter_name.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string `json:"path"`
				Linters string `json:"linters"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			linterSet := parseLinterSet(args.Linters)
			var results []lintResult

			for _, l := range linterSet {
				res := runLinter(ctx, l, args.Path)
				results = append(results, res...)
			}

			// Build structured output
			if len(results) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "No issues found across all linters."}},
					Details: map[string]any{"path": args.Path, "linters": linterSet, "issue_count": 0, "success": true},
				}, nil
			}

			var lines []string
			for _, r := range results {
				lines = append(lines, fmt.Sprintf("%s:%d\t[%s]\t%s\t(%s)",
					r.File, r.Line, r.Severity, r.Message, r.LinterName))
			}

			output := strings.Join(lines, "\n")
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"path":        args.Path,
					"linters":     linterSet,
					"issue_count": len(results),
					"issues":      results,
					"success":     false,
				},
			}, nil
		},
	}
}

// lintResult represents a single lint finding.
type lintResult struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	Severity   string `json:"severity"` // error, warning, info
	Message    string `json:"message"`
	LinterName string `json:"linter_name"`
}

func parseLinterSet(linterStr string) []string {
	if linterStr == "" {
		return []string{"govet"}
	}
	parts := strings.Split(linterStr, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	if len(result) == 0 {
		return []string{"govet"}
	}
	return result
}

func runLinter(ctx context.Context, linter, path string) []lintResult {
	switch linter {
	case "govet":
		return runGoVet(ctx, path)
	case "gofmt":
		return runGofmt(ctx, path)
	case "goimports":
		return runGoImports(ctx, path)
	case "staticcheck":
		return runStaticcheck(ctx, path)
	case "golangci-lint":
		return runGolangciLint(ctx, path)
	default:
		return []lintResult{{
			File:       path,
			Line:       0,
			Severity:   "warning",
			Message:    fmt.Sprintf("Unknown linter: %s", linter),
			LinterName: "lint_check",
		}}
	}
}

func runGoVet(ctx context.Context, path string) []lintResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "go", "vet", path)
	cmd.Dir = WorkspaceRoot
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err == nil && text == "" {
		return nil
	}

	return parseGoToolOutput(text, "govet")
}

func runGofmt(ctx context.Context, path string) []lintResult {
	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "gofmt", "-l", path)
	cmd.Dir = WorkspaceRoot
	output, _ := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}

	var results []lintResult
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		results = append(results, lintResult{
			File:       line,
			Line:       0,
			Severity:   "warning",
			Message:    "File needs formatting (not gofmt'd)",
			LinterName: "gofmt",
		})
	}
	return results
}

func runGoImports(ctx context.Context, path string) []lintResult {
	_, err := exec.LookPath("goimports")
	if err != nil {
		return []lintResult{{
			File:       path,
			Line:       0,
			Severity:   "warning",
			Message:    "goimports not installed; install via: go install golang.org/x/tools/cmd/goimports@latest",
			LinterName: "goimports",
		}}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "goimports", "-l", path)
	cmd.Dir = WorkspaceRoot
	output, _ := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil
	}

	var results []lintResult
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		results = append(results, lintResult{
			File:       line,
			Line:       0,
			Severity:   "warning",
			Message:    "Import formatting needed",
			LinterName: "goimports",
		})
	}
	return results
}

func runStaticcheck(ctx context.Context, path string) []lintResult {
	_, err := exec.LookPath("staticcheck")
	if err != nil {
		return []lintResult{{
			File:       path,
			Line:       0,
			Severity:   "warning",
			Message:    "staticcheck not installed; install via: go install honnef.co/go/tools/cmd/staticcheck@latest",
			LinterName: "staticcheck",
		}}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "staticcheck", path)
	cmd.Dir = WorkspaceRoot
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err == nil && text == "" {
		return nil
	}

	return parseGoToolOutput(text, "staticcheck")
}

func runGolangciLint(ctx context.Context, path string) []lintResult {
	_, err := exec.LookPath("golangci-lint")
	if err != nil {
		return []lintResult{{
			File:       path,
			Line:       0,
			Severity:   "warning",
			Message:    "golangci-lint not installed; install via: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest",
			LinterName: "golangci-lint",
		}}
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "golangci-lint", "run", "--timeout=60s", path)
	cmd.Dir = WorkspaceRoot
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err == nil && text == "" {
		return nil
	}

	return parseGoToolOutput(text, "golangci-lint")
}

// parseGoToolOutput parses standard Go tool output format:
//   file:line:col: message
func parseGoToolOutput(text, linterName string) []lintResult {
	var results []lintResult
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		file, lineNum, msg := parseGoOutputLine(line)
		results = append(results, lintResult{
			File:       file,
			Line:       lineNum,
			Severity:   classifySeverity(msg),
			Message:    msg,
			LinterName: linterName,
		})
	}
	return results
}

func parseGoOutputLine(line string) (file string, lineNum int, msg string) {
	// Find first colon that starts a line number
	idx := strings.Index(line, ":")
	if idx < 0 {
		return line, 0, line
	}

	file = line[:idx]
	rest := line[idx+1:]

	// Try to parse line number
	var lineStr string
	for i, c := range rest {
		if c >= '0' && c <= '9' {
			lineStr += string(c)
		} else {
			rest = rest[i:]
			break
		}
	}

	if lineStr != "" {
		fmt.Sscanf(lineStr, "%d", &lineNum)
	}

	// Rest may contain ":col: message" — strip the column
	rest = strings.TrimSpace(rest)
	if len(rest) > 0 && rest[0] == ':' {
		rest = rest[1:]
		// Try to skip column number
		var colStr string
		for i, c := range rest {
			if c >= '0' && c <= '9' {
				colStr += string(c)
			} else {
				rest = strings.TrimSpace(rest[i:])
				break
			}
		}
	}
	// Also strip leading ": " if present
	rest = strings.TrimPrefix(rest, ": ")
	rest = strings.TrimSpace(rest)

	return file, lineNum, rest
}

func classifySeverity(msg string) string {
	msg = strings.ToLower(msg)
	if strings.Contains(msg, "error") || strings.Contains(msg, "fatal") {
		return "error"
	}
	if strings.Contains(msg, "warning") || strings.Contains(msg, "deprecated") {
		return "warning"
	}
	return "info"
}