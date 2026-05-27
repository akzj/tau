package tools

import (
	"bytes"
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

	"github.com/akzj/tau/core"
)

// FindDeadcodeTool creates a tool that detects potentially dead (unused) code.
func FindDeadcodeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path (default: workspace root)"},
			"include_tests": {"type": "boolean", "description": "Include test files (default: false)"}
		}
	}`)

	return core.Tool{
		Name:        "find_deadcode",
		Description: "Find potentially dead (unused) Go code using AST analysis and go vet.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path         string `json:"path"`
				IncludeTests bool   `json:"include_tests"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			searchDir := WorkspaceRoot
			if args.Path != "" {
				var err error
				searchDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			var result strings.Builder
			result.WriteString(fmt.Sprintf("## Dead Code Report: %s\n\n", searchDir))

			// Step 1: AST-based unused function detection.
			unusedFuncs := findUnusedFunctions(searchDir, args.IncludeTests)
			result.WriteString(fmt.Sprintf("### Unused Functions (%d)\n", len(unusedFuncs)))
			if len(unusedFuncs) > 0 {
				for _, uf := range unusedFuncs {
					result.WriteString(fmt.Sprintf("- %s\n", uf))
				}
			} else {
				result.WriteString("No potentially unused functions detected.\n")
			}

			// Step 2: go vet.
			vetOutput := runDeadcodeVet(ctx, searchDir)
			vetCount := countVetIssues(vetOutput)
			result.WriteString(fmt.Sprintf("\n### go vet Issues (%d)\n", vetCount))
			if vetOutput != "" {
				result.WriteString(vetOutput)
			} else {
				result.WriteString("No issues found.\n")
			}

			// Summary.
			result.WriteString(fmt.Sprintf("\n### Summary\n%d potentially dead functions, %d vet issues\n",
				len(unusedFuncs), vetCount))

			output := result.String()
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"path":             searchDir,
					"unused_functions": len(unusedFuncs),
				},
			}, nil
		},
	}
}

// funcEntry records a non-exported function declaration for dead-code analysis.
type funcEntry struct {
	file    string
	line    int
	name    string
	pkgName string
}

// findUnusedFunctions walks .go files under dir, collecting non-exported
// top-level function declarations and every direct callee name (keyed by
// "pkg.func"), then reports every declaration whose key is absent from the
// call set.  Methods, init, and main are excluded.
func findUnusedFunctions(dir string, includeTests bool) []string {
	fset := token.NewFileSet()
	var funcs []funcEntry
	calls := make(map[string]bool) // "pkgName.funcName" → true

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		if !includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil // skip unparseable files
		}
		pkgName := file.Name.Name
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if node.Name != nil && node.Recv == nil {
					name := node.Name.Name
					if !token.IsExported(name) && name != "init" && name != "main" {
						funcs = append(funcs, funcEntry{
							file:    path,
							line:    fset.Position(node.Pos()).Line,
							name:    name,
							pkgName: pkgName,
						})
					}
				}
			case *ast.CallExpr:
				if ident, ok := node.Fun.(*ast.Ident); ok {
					calls[pkgName+"."+ident.Name] = true
				}
			}
			return true
		})
		return nil
	})

	var unused []string
	for _, f := range funcs {
		if !calls[f.pkgName+"."+f.name] {
			rel, _ := filepath.Rel(dir, f.file)
			unused = append(unused, fmt.Sprintf("%s:%d: func %s() — never called in codebase", rel, f.line, f.name))
		}
	}
	return unused
}

// runDeadcodeVet executes "go vet <dir>" and returns the combined stdout+stderr.
func runDeadcodeVet(ctx context.Context, dir string) string {
	cmd := exec.CommandContext(ctx, "go", "vet", "./...")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Run()
	out := strings.TrimSpace(stdout.String() + "\n" + stderr.String())
	return strings.TrimSpace(out)
}

// countVetIssues returns a rough count of issue lines in go vet output.
func countVetIssues(output string) int {
	if output == "" {
		return 0
	}
	lines := strings.Split(output, "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "vet: ") {
			continue
		}
		count++
	}
	return count
}

// Ensure filepath.Rel is used (prevents unused import if Rel only used in func above).
var _ = filepath.Rel
var _ = os.MkdirAll
