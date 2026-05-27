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

// CheckDocsTool creates a documentation coverage checker.
//
// Parameters:
//
//	path (string, required) — file or directory path to check
//	type (string, optional) — what to check: package, functions, exported (default: exported)
func CheckDocsTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "File or directory path to check documentation"},
			"type": {"type": "string", "description": "What to check: package, functions, exported (default: exported)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "check_docs",
		Description: "Documentation coverage checker. Analyzes Go source for undocumented exported symbols, packages without doc comments, and missing godoc coverage. Returns symbol+file+missing_doc.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				CType  string `json:"type"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.CType == "" {
				args.CType = "exported"
			}

			missing := checkDocs(args.Path, args.CType)

			if len(missing) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Documentation check passed: all %s symbols documented.", args.CType)}},
					Details: map[string]any{"path": args.Path, "type": args.CType, "missing_count": 0, "success": true},
				}, nil
			}

			var lines []string
			for _, m := range missing {
				lines = append(lines, fmt.Sprintf("%s:%s\tmissing documentation for %s", m.File, m.Symbol, m.Kind))
			}

			output := strings.Join(lines, "\n")
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"path":          args.Path,
					"type":          args.CType,
					"missing_count": len(missing),
					"missing":       missing,
					"success":       false,
				},
			}, nil
		},
	}
}

type docMissing struct {
	File   string `json:"file"`
	Symbol string `json:"symbol"`
	Kind   string `json:"kind"` // function, method, type, const, var, package
}

func checkDocs(path string, checkType string) []docMissing {
	fi, err := os.Stat(path)
	if err != nil {
		return []docMissing{{File: path, Symbol: "<error>", Kind: "error"}}
	}

	if !fi.IsDir() {
		return checkFileDocs(path, checkType)
	}

	return checkDirDocs(path, checkType)
}

func checkDirDocs(dir string, checkType string) []docMissing {
	var missing []docMissing

	// Check for doc.go at package level
	if checkType == "package" || checkType == "exported" {
		hasDocGo := false
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.Name() == "doc.go" {
				hasDocGo = true
				break
			}
		}
		if !hasDocGo {
			// Check if any file has a package doc comment
			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
				return strings.HasSuffix(fi.Name(), ".go") && !strings.HasSuffix(fi.Name(), "_test.go")
			}, parser.ParseComments)
			if err != nil {
				return []docMissing{{File: dir, Symbol: "<parse error>", Kind: "package"}}
			}
			foundDoc := false
			for _, pkg := range pkgs {
				for _, f := range pkg.Files {
					if f.Doc != nil && f.Doc.Text() != "" {
						foundDoc = true
						break
					}
				}
				if foundDoc {
					break
				}
			}
			if !foundDoc {
				pkgName := filepath.Base(dir)
				if len(pkgs) > 0 {
					for k := range pkgs {
						pkgName = k
						break
					}
				}
				missing = append(missing, docMissing{
					File:   filepath.Join(dir, "<package>"),
					Symbol: pkgName,
					Kind:   "package",
				})
			}
		}
	}

	if checkType == "package" {
		return missing
	}

	// Check all Go files in directory for function/exported docs
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		missing = append(missing, checkFileDocs(p, checkType)...)
		return nil
	})
	if err != nil {
		missing = append(missing, docMissing{File: dir, Symbol: "<walk error>", Kind: "error"})
	}

	return missing
}

func checkFileDocs(filePath string, checkType string) []docMissing {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return []docMissing{{File: filePath, Symbol: "<parse error>", Kind: "error"}}
	}

	if checkType == "exported" {
		return checkExportedDocs(f, filePath)
	}
	if checkType == "functions" {
		return checkFunctionDocs(f, filePath)
	}
	return checkExportedDocs(f, filePath)
}

func checkExportedDocs(f *ast.File, filePath string) []docMissing {
	var missing []docMissing

	// Check exported functions and methods
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.IsExported() && d.Doc == nil {
				kind := "function"
				if d.Recv != nil {
					kind = "method"
				}
				missing = append(missing, docMissing{
					File:   filePath,
					Symbol: d.Name.Name,
					Kind:   kind,
				})
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.IsExported() && d.Doc == nil {
						missing = append(missing, docMissing{
							File:   filePath,
							Symbol: s.Name.Name,
							Kind:   "type",
						})
					}
				case *ast.ValueSpec:
					for _, name := range s.Names {
						if name.IsExported() && d.Doc == nil {
							kind := "var"
							if name.Obj != nil && name.Obj.Kind == ast.Con {
								kind = "const"
							}
							missing = append(missing, docMissing{
								File:   filePath,
								Symbol: name.Name,
								Kind:   kind,
							})
						}
					}
				}
			}
		}
	}

	return missing
}

func checkFunctionDocs(f *ast.File, filePath string) []docMissing {
	var missing []docMissing

	for _, decl := range f.Decls {
		d, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if d.Doc == nil {
			missing = append(missing, docMissing{
				File:   filePath,
				Symbol: d.Name.Name,
				Kind:   "function",
			})
		}
	}

	return missing
}