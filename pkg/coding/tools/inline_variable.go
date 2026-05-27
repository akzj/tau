package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// InlineVariableTool creates a tool for inlining variable usage.
//
// Parameters:
//
//	file          (string, required) — file path relative to workspace root
//	variable_name (string, required) — variable name to inline
func InlineVariableTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file":          {"type": "string", "description": "File path relative to workspace root"},
			"variable_name": {"type": "string", "description": "Variable name to inline"}
		},
		"required": ["file", "variable_name"]
	}`)

	return core.Tool{
		Name:        "inline_variable",
		Description: "Inline variable usage. Replaces all references to a variable with its initializer value and removes the declaration. Returns a diff summary.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File         string `json:"file"`
				VariableName string `json:"variable_name"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}
			if args.VariableName == "" {
				return core.ToolResult{}, fmt.Errorf("variable_name required")
			}

			fullPath, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, err
			}

			result, err := inlineVariable(fullPath, args.VariableName)
			if err != nil {
				return core.ToolResult{}, err
			}

			err = os.WriteFile(fullPath, []byte(result.NewContent), 0644)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("write file: %w", err)
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Inlined variable: %s\n", args.VariableName))
			output.WriteString(fmt.Sprintf("References inlined: %d\n", result.RefsInlined))
			output.WriteString(fmt.Sprintf("Initializer: %s\n", result.Initializer))
			output.WriteString("--- New code excerpt ---\n")
			output.WriteString(result.NewContent)

			resultText := output.String()
			if len(resultText) > OutputCap {
				resultText = resultText[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: resultText}},
				Details: map[string]any{
					"file":          fullPath,
					"variable_name": args.VariableName,
					"refs_inlined":  result.RefsInlined,
					"initializer":   result.Initializer,
				},
			}, nil
		},
	}
}

type inlineResult struct {
	NewContent  string
	Initializer string
	RefsInlined int
}

func inlineVariable(fullPath, varName string) (*inlineResult, error) {
	fset := token.NewFileSet()
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	file, err := parser.ParseFile(fset, fullPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	// Find the variable declaration and its initializer
	var declNode *ast.ValueSpec
	var initExpr ast.Expr
	ast.Inspect(file, func(n ast.Node) bool {
		switch nd := n.(type) {
		case *ast.FuncDecl:
			// Only look inside functions
			if nd.Body == nil {
				return false
			}
		case *ast.GenDecl:
			// Package-level var — skip for simplicity
			return false
		}
		return true
	})

	// Simpler approach: walk the entire file looking for the variable declaration
	ast.Inspect(file, func(n ast.Node) bool {
		if declNode != nil {
			return false
		}
		if assign, ok := n.(*ast.AssignStmt); ok {
			if assign.Tok == token.DEFINE || assign.Tok == token.ASSIGN {
				for i, lhs := range assign.Lhs {
					if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
						if i < len(assign.Rhs) {
							declNode = &ast.ValueSpec{
								Names:  []*ast.Ident{ident},
								Values: []ast.Expr{assign.Rhs[i]},
							}
							initExpr = assign.Rhs[i]
							return false
						}
					}
				}
			}
		}
		return true
	})

	if declNode == nil {
		return nil, fmt.Errorf("variable %q not found or not a simple declaration", varName)
	}

	// Extract the initializer text
	initText := extractExprText(content, fset, initExpr)
	if initText == "" {
		return nil, fmt.Errorf("could not extract initializer for %q", varName)
	}

	// Now replace all usages of varName with the initializer text
	// except for the declaration itself.
	declPos := fset.Position(declNode.Names[0].Pos())
	declEnd := fset.Position(declNode.Names[0].End())

	type replaceRange struct {
		start int
		end   int
		text  string
	}

	var ranges []replaceRange
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == varName {
			pos := fset.Position(ident.Pos())
			// Skip the declaration identifier itself
			if pos.Offset >= declPos.Offset && pos.Offset < declEnd.Offset {
				return true
			}
			ranges = append(ranges, replaceRange{
				start: pos.Offset,
				end:   fset.Position(ident.End()).Offset,
				text:  initText,
			})
		}
		return true
	})

	if len(ranges) == 0 {
		return nil, fmt.Errorf("no references to inline for variable %q", varName)
	}

	// Sort by offset descending
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start > ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}

	// Apply replacements
	result := string(content)
	for _, r := range ranges {
		result = result[:r.start] + r.text + result[r.end:]
	}

	// Remove the declaration line: find the assignment statement and remove it
	// We re-parse and remove the entire assignment (or just the var if multi-var)
	result = removeVarDeclaration(result, varName, bool(declNode.Names[0].Obj != nil))

	// Format
	fmtCode, err := format.Source([]byte(result))
	if err != nil {
		return &inlineResult{
			NewContent:  result,
			Initializer: initText,
			RefsInlined: len(ranges),
		}, nil
	}

	return &inlineResult{
		NewContent:  string(fmtCode),
		Initializer: initText,
		RefsInlined: len(ranges),
	}, nil
}

func extractExprText(src []byte, fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	pos := fset.Position(expr.Pos())
	end := fset.Position(expr.End())
	if pos.Offset >= 0 && end.Offset <= len(src) && pos.Offset < end.Offset {
		return string(src[pos.Offset:end.Offset])
	}
	return ""
}

func removeVarDeclaration(src, varName string, hadObj bool) string {
	// Parse to find and remove the := or = statement
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return src
	}

	var removeStart, removeEnd int
	ast.Inspect(file, func(n ast.Node) bool {
		if removeStart > 0 {
			return false
		}
		if assign, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assign.Lhs {
				if ident, ok := lhs.(*ast.Ident); ok && ident.Name == varName {
					pos := fset.Position(assign.Pos())
					end := fset.Position(assign.End())
					removeStart = pos.Offset
					removeEnd = end.Offset
					return false
				}
			}
		}
		return true
	})

	if removeStart > 0 && removeEnd > removeStart {
		// Also remove trailing newline/whitespace
		for removeEnd < len(src) && (src[removeEnd] == '\n' || src[removeEnd] == '\r') {
			removeEnd++
		}
		return src[:removeStart] + src[removeEnd:]
	}
	return src
}
