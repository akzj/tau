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

// ListSymbolsTool creates a tool that lists Go symbols in source files.
func ListSymbolsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path (default: workspace root)"},
			"type_filter": {"type": "string", "description": "Comma-separated filter: func,type,var,const,interface. Empty = all."}
		}
	}`)

	return core.Tool{
		Name:        "list_symbols",
		Description: "List Go symbols (functions, types, variables, constants, interfaces) in Go source files. Skips test files by default.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path       string `json:"path"`
				TypeFilter string `json:"type_filter"`
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

			// Parse type filter into a set
			filter := make(map[string]bool)
			if args.TypeFilter != "" {
				for _, f := range strings.Split(args.TypeFilter, ",") {
					filter[strings.TrimSpace(f)] = true
				}
			}

			fset := token.NewFileSet()
			var results []string

			err := filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
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

				file, parseErr := parser.ParseFile(fset, path, nil, 0)
				if parseErr != nil {
					return nil
				}

				relPath, _ := filepath.Rel(WorkspaceRoot, path)

				ast.Inspect(file, func(n ast.Node) bool {
					switch decl := n.(type) {
					case *ast.FuncDecl:
						if len(filter) > 0 && !filter["func"] {
							return true
						}
						kind := "func"
						name := decl.Name.Name
						if decl.Recv != nil && len(decl.Recv.List) > 0 {
							kind = "method"
							recvType := typeExprString(decl.Recv.List[0].Type)
							name = fmt.Sprintf("(%s).%s", recvType, decl.Name.Name)
						}
						pos := fset.Position(decl.Pos())
						results = append(results, fmt.Sprintf("%s:%d: %s (%s) %s",
							relPath, pos.Line, name, kind, exportTag(name)))
						return true

					case *ast.GenDecl:
						if decl.Tok == token.IMPORT {
							return true
						}
						for _, spec := range decl.Specs {
							switch s := spec.(type) {
							case *ast.TypeSpec:
								if len(filter) > 0 {
									_, wantType := filter["type"]
									_, wantIface := filter["interface"]
									if !wantType && !wantIface {
										continue
									}
								}
								kind := "type"
								if _, isIface := s.Type.(*ast.InterfaceType); isIface {
									kind = "interface"
								}
								pos := fset.Position(decl.Pos())
								results = append(results, fmt.Sprintf("%s:%d: %s (%s) %s",
									relPath, pos.Line, s.Name.Name, kind, exportTag(s.Name.Name)))
							case *ast.ValueSpec:
								kind := "var"
								if decl.Tok == token.CONST {
									kind = "const"
								}
								if len(filter) > 0 && !filter[kind] {
									continue
								}
								pos := fset.Position(decl.Pos())
								for _, name := range s.Names {
									results = append(results, fmt.Sprintf("%s:%d: %s (%s) %s",
										relPath, pos.Line, name.Name, kind, exportTag(name.Name)))
								}
							}
						}
					}
					return true
				})
				return nil
			})

			if err != nil {
				return core.ToolResult{}, err
			}
			if len(results) == 0 {
				return core.ToolResult{Content: []core.Content{{Type: "text", Text: "No symbols found."}}}, nil
			}

			output := strings.Join(results, "\n")
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"count": len(results), "path": searchDir},
			}, nil
		},
	}
}

// typeExprString returns a string representation of a type expression.
func typeExprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + typeExprString(t.X)
	case *ast.SelectorExpr:
		return typeExprString(t.X) + "." + t.Sel.Name
	case *ast.IndexExpr:
		return typeExprString(t.X) + "[...]"
	case *ast.ArrayType:
		return "[]" + typeExprString(t.Elt)
	}
	return "<type>"
}

// exportTag returns "[exported]" if the symbol name is exported (starts with uppercase).
func exportTag(name string) string {
	clean := name
	if idx := strings.LastIndex(clean, ")."); idx >= 0 {
		clean = clean[idx+2:]
	} else if idx := strings.LastIndex(clean, "."); idx >= 0 {
		clean = clean[idx+1:]
	}
	if len(clean) > 0 && clean[0] >= 'A' && clean[0] <= 'Z' {
		return "[exported]"
	}
	return ""
}
