package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// CallHierarchyTool creates a tool that analyzes function call hierarchy.
func CallHierarchyTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"function_name": {"type": "string", "description": "Function name to analyze"},
			"path": {"type": "string", "description": "Directory path (default: workspace root)"},
			"direction": {"type": "string", "description": "callers, callees, or both (default: both)"},
			"max_depth": {"type": "integer", "description": "Max recursion depth (default: 3)"},
			"include_external": {"type": "boolean", "description": "Include external packages (default: false)"}
		},
		"required": ["function_name"]
	}`)

	return core.Tool{
		Name:        "call_hierarchy",
		Description: "Analyze function call hierarchy — find callers and callees of a function. Callees use AST traversal; callers use regex grep + AST verification.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FunctionName    string `json:"function_name"`
				Path            string `json:"path"`
				Direction       string `json:"direction"`
				MaxDepth        int    `json:"max_depth"`
				IncludeExternal bool   `json:"include_external"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.FunctionName == "" {
				return core.ToolResult{}, fmt.Errorf("function_name required")
			}
			if args.Direction == "" {
				args.Direction = "both"
			}
			if args.MaxDepth <= 0 {
				args.MaxDepth = 3
			}

			searchDir := WorkspaceRoot
			if args.Path != "" {
				var err error
				searchDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			var output strings.Builder

			// ── Callees ──
			if args.Direction == "callees" || args.Direction == "both" {
				calleeResult := findCallees(searchDir, args.FunctionName, args.MaxDepth, args.IncludeExternal)
				if calleeResult != "" {
					output.WriteString("=== CALLEES (calls made by function) ===\n")
					output.WriteString(calleeResult)
				} else {
					output.WriteString("=== CALLEES ===\nFunction not found or makes no calls.\n")
				}
			}

			// ── Callers ──
			if args.Direction == "callers" || args.Direction == "both" {
				if output.Len() > 0 {
					output.WriteString("\n")
				}
				callerResult := findCallers(searchDir, args.FunctionName)
				if callerResult != "" {
					output.WriteString("=== CALLERS (sites that call this function) ===\n")
					output.WriteString(callerResult)
				} else {
					output.WriteString("=== CALLERS ===\nNo callers found.\n")
				}
			}

			result := output.String()
			if len(result) > OutputCap {
				result = result[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: strings.TrimSpace(result)}},
				Details: map[string]any{"function": args.FunctionName, "direction": args.Direction},
			}, nil
		},
	}
}

// ── Callee analysis ──

// calleeInfo holds a callee entry: name and file:line where it's called.
type calleeInfo struct {
	name string
	file string
	line int
}

// findCallees finds all callees of a function recursively up to maxDepth.
func findCallees(root string, funcName string, maxDepth int, includeExternal bool) string {
	var seen = make(map[string]bool)
	var lines []string
	findCalleesRec(root, funcName, maxDepth, 0, seen, includeExternal, &lines)
	return strings.Join(lines, "\n")
}

func findCalleesRec(root string, funcName string, maxDepth int, depth int, seen map[string]bool, includeExternal bool, lines *[]string) {
	if depth >= maxDepth {
		return
	}
	key := fmt.Sprintf("%s@%d", funcName, depth)
	if seen[key] {
		return
	}
	seen[key] = true

	prefix := strings.Repeat("  ", depth)
	if depth > 0 {
		prefix += "→ "
	}

	// Find the function definition across all Go files
	fset := token.NewFileSet()
	var fnDecl *ast.FuncDecl
	var fnFile string
	var fnLine int

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || fnDecl != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				if fd.Name.Name == funcName {
					fnDecl = fd
					pos := fset.Position(fd.Pos())
					fnFile, _ = filepath.Rel(root, pos.Filename)
					fnLine = pos.Line
					return false
				}
			}
			return true
		})
		return nil
	})

	if fnDecl == nil {
		relPath := ""
		// Special case: if this is the root call, report not found
		if depth == 0 {
			*lines = append(*lines, fmt.Sprintf("%s: function not found in workspace", funcName))
		}
		_ = relPath
		return
	}

	if depth == 0 {
		*lines = append(*lines, fmt.Sprintf("%s%s (%s:%d)", prefix, funcName, fnFile, fnLine))
		prefix += "  "
	} else {
		*lines = append(*lines, fmt.Sprintf("%s%s (%s:%d)", prefix, funcName, fnFile, fnLine))
	}

	// Collect callees from function body
	callees := collectCalleesFromFunc(fnDecl, fset)
	for _, c := range callees {
		rel, _ := filepath.Rel(root, c.file)
		*lines = append(*lines, fmt.Sprintf("%s→ %s (%s:%d)", prefix, c.name, rel, c.line))
		// Recurse into callee
		findCalleesRec(root, c.name, maxDepth, depth+1, seen, includeExternal, lines)
	}
}

// collectCalleesFromFunc extracts all function calls from a function's body.
func collectCalleesFromFunc(fn *ast.FuncDecl, fset *token.FileSet) []calleeInfo {
	var callees []calleeInfo
	seen := make(map[string]bool)

	if fn.Body == nil {
		return callees
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callExprName(call)
		if name == "" || seen[name] {
			return true
		}
		seen[name] = true
		pos := fset.Position(call.Pos())
		callees = append(callees, calleeInfo{name: name, file: pos.Filename, line: pos.Line})
		return true
	})
	return callees
}

// callExprName extracts a human-readable name from a CallExpr.
func callExprName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return exprName(fun.X) + "." + fun.Sel.Name
	default:
		return ""
	}
}

// exprName extracts a name from an arbitrary expression (for SelectorExpr.X).
func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprName(e.X) + "." + e.Sel.Name
	case *ast.CallExpr:
		return callExprName(e)
	default:
		return ""
	}
}

// ── Caller analysis ──

// findCallers uses regex grep to find all call sites of a function.
func findCallers(root string, funcName string) string {
	fset := token.NewFileSet()
	pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(funcName) + `\s*\(`)
	var results []string

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		matches := pattern.FindAllIndex(content, -1)
		if len(matches) == 0 {
			return nil
		}

		// Parse file to verify each match is a call (not a declaration)
		file, parseErr := parser.ParseFile(fset, path, content, 0)
		if parseErr != nil {
			return nil
		}

		relPath, _ := filepath.Rel(root, path)

		for _, m := range matches {
			offset := m[0]
			// Check if this position is inside a FuncDecl (which would be the declaration)
			inDecl := false
			ast.Inspect(file, func(n ast.Node) bool {
				if fd, ok := n.(*ast.FuncDecl); ok {
					if fd.Name.Name == funcName {
						pos := fset.Position(fd.Pos())
						endPos := fset.Position(fd.End())
						byteOff := int(pos.Offset)
						byteEnd := int(endPos.Offset)
						if offset >= byteOff && offset < byteEnd {
							inDecl = true
							return false
						}
					}
				}
				return true
			})

			if !inDecl {
				pos := fset.Position(token.Pos(int(file.Pos()) + offset))
				results = append(results, fmt.Sprintf("%s:%d: call to %s()", relPath, pos.Line, funcName))
			}
		}
		return nil
	})

	if len(results) == 0 {
		return ""
	}
	return strings.Join(results, "\n")
}
