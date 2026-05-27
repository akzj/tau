package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// ComplexityReportTool creates a cyclomatic complexity reporter.
//
// Parameters:
//
//	path      (string, required) — file or directory path to analyze
//	threshold (number, optional) — complexity threshold (default: 15)
func ComplexityReportTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory to analyze for complexity"},
			"threshold": {"type": "number", "description": "Complexity threshold (default: 15)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "complexity_report",
		Description: "Cyclomatic complexity reporter. Analyzes Go functions, computes cyclomatic complexity via AST analysis, flags functions exceeding threshold. Returns function+complexity+file+exceeds_threshold.",
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
				args.Threshold = 15
			}

			// Try gocyclo first
			if _, err := exec.LookPath("gocyclo"); err == nil {
				return complexityWithGocyclo(ctx, args.Path, int(args.Threshold))
			}

			// Fall back to AST-based analysis
			return complexityWithAST(ctx, args.Path, int(args.Threshold))
		},
	}
}

type complexityEntry struct {
	Function          string `json:"function"`
	Complexity        int    `json:"complexity"`
	File              string `json:"file"`
	Line              int    `json:"line"`
	ExceedsThreshold  bool   `json:"exceeds_threshold"`
}

func complexityWithGocyclo(ctx context.Context, path string, threshold int) (core.ToolResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "gocyclo", "-over", fmt.Sprintf("%d", threshold-1), path)
	cmd.Dir = WorkspaceRoot
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil && text == "" {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("gocyclo failed: %v", err)}},
			Details: map[string]any{"path": path, "success": false},
		}, err
	}

	if text == "" {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("No functions exceed complexity threshold of %d.", threshold)}},
			Details: map[string]any{"path": path, "threshold": threshold, "exceeding_count": 0, "success": true},
		}, nil
	}

	// Parse gocyclo output: "complexity function file:line:col"
	entries := parseGocycloOutput(text, threshold)
	return buildComplexityResult(entries, threshold), nil
}

func parseGocycloOutput(text string, threshold int) []complexityEntry {
	var entries []complexityEntry
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		complexity := 0
		fmt.Sscanf(parts[0], "%d", &complexity)
		funcName := parts[1]
		loc := parts[2]
		file, lineNum := parseLocation(loc)

		entries = append(entries, complexityEntry{
			Function:         funcName,
			Complexity:       complexity,
			File:             file,
			Line:             lineNum,
			ExceedsThreshold: complexity > threshold,
		})
	}
	return entries
}

func parseLocation(loc string) (string, int) {
	// Format: file:line:col or file:line
	idx := strings.LastIndex(loc, ":")
	if idx < 0 {
		return loc, 0
	}
	file := loc[:idx]
	rest := loc[idx+1:]
	// rest may be "line:col" or just "line"
	colonIdx := strings.Index(rest, ":")
	lineStr := rest
	if colonIdx >= 0 {
		lineStr = rest[:colonIdx]
	}
	lineNum := 0
	fmt.Sscanf(lineStr, "%d", &lineNum)
	return file, lineNum
}

func complexityWithAST(ctx context.Context, path string, threshold int) (core.ToolResult, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("cannot stat path: %w", err)
	}

	var goFiles []string
	if fi.IsDir() {
		filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
				goFiles = append(goFiles, p)
			}
			return nil
		})
	} else if strings.HasSuffix(path, ".go") {
		goFiles = append(goFiles, path)
	}

	var allEntries []complexityEntry
	for _, gf := range goFiles {
		entries, err := analyzeFileComplexity(gf)
		if err != nil {
			continue
		}
		for _, e := range entries {
			e.ExceedsThreshold = e.Complexity > threshold
			allEntries = append(allEntries, e)
		}
	}

	// Filter to only exceeding if threshold used
	var exceeding []complexityEntry
	for _, e := range allEntries {
		if e.Complexity > threshold {
			exceeding = append(exceeding, e)
		}
	}

	return buildComplexityResult(exceeding, threshold), nil
}

func analyzeFileComplexity(filePath string) ([]complexityEntry, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, 0)
	if err != nil {
		return nil, err
	}

	var entries []complexityEntry
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		c := computeCyclomaticComplexity(fd)
		pos := fset.Position(fd.Pos())
		entries = append(entries, complexityEntry{
			Function:  fd.Name.Name,
			Complexity: c,
			File:      filePath,
			Line:      pos.Line,
		})
	}

	return entries, nil
}

func computeCyclomaticComplexity(fd *ast.FuncDecl) int {
	// Start at 1 (base complexity)
	c := 1
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt:
			c++
		case *ast.ForStmt:
			c++
		case *ast.RangeStmt:
			c++
		case *ast.CaseClause:
			if n.List != nil {
				c++
			}
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				c++
			}
		}
		return true
	})
	return c
}

func buildComplexityResult(entries []complexityEntry, threshold int) core.ToolResult {
	if len(entries) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("No functions exceed complexity threshold of %d.", threshold)}},
			Details: map[string]any{"path": "", "threshold": threshold, "exceeding_count": 0, "success": true},
		}
	}

	var lines []string
	exceedingCount := 0
	for _, e := range entries {
		if e.ExceedsThreshold {
			exceedingCount++
		}
		marker := " "
		if e.ExceedsThreshold {
			marker = "!"
		}
		lines = append(lines, fmt.Sprintf("%s %d\t%s\t%s:%d",
			marker, e.Complexity, e.Function, e.File, e.Line))
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"threshold":       threshold,
			"exceeding_count": exceedingCount,
			"total_functions": len(entries),
			"success":         exceedingCount == 0,
			"entries":         entries,
		},
	}
}