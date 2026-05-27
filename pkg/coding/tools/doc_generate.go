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
	"strings"

	"github.com/akzj/tau/core"
)

// DocGenerateTool creates a code → markdown documentation generator.
//
// Parameters:
//
//	path   (string, required) — Go file or directory to document
//	output (string, optional) — output format: markdown (default) | plain
//
// Extracts function signatures, comments, type definitions, and
// generates structured .md documentation.
func DocGenerateTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Go file or directory path to document"},
			"output": {"type": "string", "description": "Output format: markdown (default) or plain"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "doc_generate",
		Description: "Generate markdown documentation from Go source: function signatures, comments, types.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				Output string `json:"output"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Output == "" {
				args.Output = "markdown"
			}

			resolved, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			info, err := os.Stat(resolved)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("stat path: %w", err)
			}

			var goFiles []string
			if info.IsDir() {
				entries, err := os.ReadDir(resolved)
				if err != nil {
					return core.ToolResult{}, err
				}
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") && !strings.HasSuffix(e.Name(), "_test.go") {
						goFiles = append(goFiles, filepath.Join(resolved, e.Name()))
					}
				}
			} else {
				if strings.HasSuffix(resolved, ".go") {
					goFiles = append(goFiles, resolved)
				} else {
					return core.ToolResult{}, fmt.Errorf("not a .go file or directory: %s", args.Path)
				}
			}

			if len(goFiles) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("(no .go files found in %s)", args.Path)}},
				}, nil
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("# Documentation: %s\n\n", args.Path))

			fset := token.NewFileSet()

			for _, f := range goFiles {
				node, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
				if err != nil {
					output.WriteString(fmt.Sprintf("_Parse error: %s: %v_\n\n", filepath.Base(f), err))
					continue
				}

				output.WriteString(fmt.Sprintf("## File: %s\n\n", filepath.Base(f)))

				// Package doc
				if node.Doc != nil {
					output.WriteString(fmt.Sprintf("**Package %s**: %s\n\n", node.Name.Name, strings.TrimSpace(node.Doc.Text())))
				} else {
					output.WriteString(fmt.Sprintf("Package: `%s`\n\n", node.Name.Name))
				}

				// Types
				for _, decl := range node.Decls {
					genDecl, ok := decl.(*ast.GenDecl)
					if !ok || genDecl.Tok != token.TYPE {
						continue
					}
					for _, spec := range genDecl.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						doc := ""
						if genDecl.Doc != nil {
							doc = genDecl.Doc.Text()
						}
						output.WriteString("### Type: `" + typeSpec.Name.Name + "`\n")
						if doc != "" {
							output.WriteString(doc + "\n\n")
						}
						// Show the type definition
						output.WriteString("```go\n")
						output.WriteString(fmt.Sprintf("type %s %s\n", typeSpec.Name.Name, formatExpr(typeSpec.Type)))
						output.WriteString("```\n\n")
					}
				}

				// Functions
				for _, decl := range node.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok {
						continue
					}

					var doc string
					if fn.Doc != nil {
						doc = fn.Doc.Text()
					}

					receiver := ""
					if fn.Recv != nil && len(fn.Recv.List) > 0 {
						r := fn.Recv.List[0]
						recvName := ""
						if len(r.Names) > 0 {
							recvName = r.Names[0].Name
						}
						recvType := formatExpr(r.Type)
						receiver = fmt.Sprintf("(%s %s) ", recvName, recvType)
					}

					sig := fmt.Sprintf("func %s%s%s", receiver, fn.Name.Name, formatFuncParams(fn.Type))
					output.WriteString("### `" + sig + "`\n")
					if doc != "" {
						output.WriteString(doc + "\n\n")
					} else {
						output.WriteString("\n")
					}
				}

				output.WriteString("---\n\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output.String()}},
				Details: map[string]any{"path": args.Path, "files": len(goFiles)},
			}, nil
		},
	}
}

func formatExpr(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + formatExpr(e.X)
	case *ast.SelectorExpr:
		return formatExpr(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + formatExpr(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", formatExpr(e.Len), formatExpr(e.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", formatExpr(e.Key), formatExpr(e.Value))
	case *ast.InterfaceType:
		if e.Methods.NumFields() == 0 {
			return "interface{}"
		}
		return "interface{...}"
	case *ast.FuncType:
		return "func" + formatFuncParams(e)
	case *ast.StructType:
		return "struct{...}"
	case *ast.BasicLit:
		return e.Value
	case *ast.ChanType:
		return "chan"
	default:
		return fmt.Sprintf("%T", expr)
	}
}

func formatFuncParams(ft *ast.FuncType) string {
	var params []string
	if ft.Params != nil {
		for _, field := range ft.Params.List {
			t := formatExpr(field.Type)
			for _, name := range field.Names {
				params = append(params, name.Name+" "+t)
			}
			if len(field.Names) == 0 {
				params = append(params, t)
			}
		}
	}

	result := "(" + strings.Join(params, ", ") + ")"

	if ft.Results != nil && len(ft.Results.List) > 0 {
		var results []string
		for _, field := range ft.Results.List {
			t := formatExpr(field.Type)
			if len(field.Names) > 0 {
				for _, name := range field.Names {
					results = append(results, name.Name+" "+t)
				}
			} else {
				results = append(results, t)
			}
		}
		if len(results) == 1 {
			result += " " + results[0]
		} else {
			result += " (" + strings.Join(results, ", ") + ")"
		}
	}
	return result
}
