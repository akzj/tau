package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// ApiSurfaceTool creates a tool that extracts the public API surface of a Go package.
func ApiSurfaceTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory path (default: workspace root)"},
			"include_internal": {"type": "boolean", "description": "Include unexported symbols (default false)"}
		}
	}`)

	return core.Tool{
		Name:        "api_surface",
		Description: "Extract exported functions, types, methods, constants, and variables from a Go package. Groups by package and kind.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path            string `json:"path"`
				IncludeInternal bool   `json:"include_internal"`
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

			// Collect .go files (recursive)
			var goFiles []string
			filepath.WalkDir(searchDir, func(path string, d os.DirEntry, err error) error {
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
				if !strings.HasSuffix(d.Name(), ".go") {
					return nil
				}
				if strings.HasSuffix(d.Name(), "_test.go") && !args.IncludeInternal {
					return nil
				}
				goFiles = append(goFiles, path)
				return nil
			})

			if len(goFiles) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "No Go source files found."}},
				}, nil
			}

			fset := token.NewFileSet()

			// Group by package directory
			type pkgInfo struct {
				dir   string
				name  string
				files []*ast.File
			}
			pkgMap := map[string]*pkgInfo{}

			for _, f := range goFiles {
				file, err := parser.ParseFile(fset, f, nil, parser.ParseComments)
				if err != nil {
					continue
				}
				pkgDir := filepath.Dir(f)
				key := pkgDir
				if pi, ok := pkgMap[key]; ok {
					pi.files = append(pi.files, file)
				} else {
					pkgMap[key] = &pkgInfo{
						dir:   pkgDir,
						name:  file.Name.Name,
						files: []*ast.File{file},
					}
				}
			}

			// Build report
			var b strings.Builder

			relDir, _ := filepath.Rel(WorkspaceRoot, searchDir)
			if relDir == "." {
				relDir = searchDir
			}
			b.WriteString(fmt.Sprintf("## API Surface: %s\n\n", relDir))

			totalPackages := 0
			totalSymbols := 0

			// Sort package keys for deterministic output
			pkgKeys := make([]string, 0, len(pkgMap))
			for k := range pkgMap {
				pkgKeys = append(pkgKeys, k)
			}
			sort.Strings(pkgKeys)

			for _, pkgKey := range pkgKeys {
				pi := pkgMap[pkgKey]
				totalPackages++

				relPkg, _ := filepath.Rel(WorkspaceRoot, pi.dir)
				b.WriteString(fmt.Sprintf("### Package: %s\n", relPkg))

				// Collect symbols by kind
				type symEntry struct {
					line  int
					file  string
					text  string
					doc   string
					order int // to preserve declaration order within kind
				}
				types := []symEntry{}
				funcs := []symEntry{}
				methods := []symEntry{}
				consts := []symEntry{}
				vars := []symEntry{}

				order := 0

				for _, file := range pi.files {
					relPath, _ := filepath.Rel(WorkspaceRoot, fset.Position(file.Pos()).Filename)

					for _, decl := range file.Decls {
						switch d := decl.(type) {
						case *ast.GenDecl:
							docText := ""
							if d.Doc != nil {
								docText = strings.TrimSpace(d.Doc.Text())
							}
							line := fset.Position(d.Pos()).Line
							for _, spec := range d.Specs {
								switch s := spec.(type) {
								case *ast.TypeSpec:
									name := s.Name.Name
									if !args.IncludeInternal && !ast.IsExported(name) {
										continue
									}
									sig := typeSpecSignature(s)
									entry := symEntry{line: line, file: relPath, text: fmt.Sprintf("type %s %s", name, sig), doc: docText, order: order}
									types = append(types, entry)
									order++

									// Collect methods defined on this type
									// (methods are parsed as FuncDecl with Recv, they'll be found in the next loop)
								case *ast.ValueSpec:
									kind := "var"
									if d.Tok == token.CONST {
										kind = "const"
									}
									specDoc := docText
									if s.Doc != nil {
										specDoc = strings.TrimSpace(s.Doc.Text())
									}
									for _, name := range s.Names {
										if !args.IncludeInternal && !ast.IsExported(name.Name) {
											continue
										}
										sig := ""
										if s.Type != nil {
											sig = typeExprString(s.Type)
										} else if len(s.Values) > 0 {
											// Try to get the value string (approximate)
											sig = "..."
										}
										if sig != "" {
											sig = " " + sig
										}
										entry := symEntry{line: line, file: relPath, text: fmt.Sprintf("%s %s%s", kind, name.Name, sig), doc: specDoc, order: order}
										if kind == "const" {
											consts = append(consts, entry)
										} else {
											vars = append(vars, entry)
										}
										order++
									}
								}
							}
						case *ast.FuncDecl:
							name := d.Name.Name
							if d.Recv != nil && len(d.Recv.List) > 0 {
								// Method: included if exported name
								if !args.IncludeInternal && !ast.IsExported(name) {
									continue
								}
								methodSig := funcSignature(d)
								docText := ""
								if d.Doc != nil {
									docText = strings.TrimSpace(d.Doc.Text())
								}
								line := fset.Position(d.Pos()).Line
								relPath, _ := filepath.Rel(WorkspaceRoot, fset.Position(d.Pos()).Filename)
								entry := symEntry{line: line, file: relPath, text: methodSig, doc: docText, order: order}
								methods = append(methods, entry)
								order++
							} else {
								// Function
								if !args.IncludeInternal && !ast.IsExported(name) {
									continue
								}
								funcSig := funcSignature(d)
								docText := ""
								if d.Doc != nil {
									docText = strings.TrimSpace(d.Doc.Text())
								}
								line := fset.Position(d.Pos()).Line
								relPath, _ := filepath.Rel(WorkspaceRoot, fset.Position(d.Pos()).Filename)
								entry := symEntry{line: line, file: relPath, text: funcSig, doc: docText, order: order}
								funcs = append(funcs, entry)
								order++
							}
						}
					}
				}

				// Now write them grouped by kind with the same structure
				sections := []struct {
					title  string
					items  []symEntry
				}{
					{"Types", types},
					{"Functions", funcs},
					{"Methods", methods},
					{"Constants", consts},
					{"Variables", vars},
				}

				pkgSymCount := 0
				for _, sec := range sections {
					if len(sec.items) == 0 {
						continue
					}
					b.WriteString(fmt.Sprintf("#### %s\n", sec.title))
					for _, item := range sec.items {
						docSuffix := ""
						if item.doc != "" {
							docSuffix = fmt.Sprintf(" — %s", firstLine(item.doc))
						}
						b.WriteString(fmt.Sprintf("- %s:%d: `%s`%s\n", item.file, item.line, item.text, docSuffix))
						pkgSymCount++
						totalSymbols++
					}
					b.WriteString("\n")
				}

				if pkgSymCount == 0 {
					b.WriteString("(no exported symbols)\n\n")
				}
			}

			// Summary
			pkgLabel := "packages"
			if totalPackages == 1 {
				pkgLabel = "package"
			}
			symLabel := "exported symbols"
			if totalSymbols == 1 {
				symLabel = "exported symbol"
			}
			b.WriteString(fmt.Sprintf("### Summary\n%d %s, %d %s\n", totalPackages, pkgLabel, totalSymbols, symLabel))

			output := b.String()
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"packages": totalPackages,
					"symbols":  totalSymbols,
					"path":     searchDir,
				},
			}, nil
		},
	}
}

// funcSignature returns a human-readable Go function/method signature.
func funcSignature(d *ast.FuncDecl) string {
	var b strings.Builder
	if d.Recv != nil && len(d.Recv.List) > 0 {
		b.WriteString("func (")
		if len(d.Recv.List[0].Names) > 0 {
			b.WriteString(d.Recv.List[0].Names[0].Name)
			b.WriteString(" ")
		}
		b.WriteString(typeExprString(d.Recv.List[0].Type))
		b.WriteString(") ")
	} else {
		b.WriteString("func ")
	}
	b.WriteString(d.Name.Name)
	b.WriteString(paramsString(d.Type.Params))
	if d.Type.Results != nil && len(d.Type.Results.List) > 0 {
		b.WriteString(" ")
		if len(d.Type.Results.List) > 1 || (len(d.Type.Results.List) == 1 && d.Type.Results.List[0].Names == nil) {
			b.WriteString(resultsString(d.Type.Results))
		} else {
			b.WriteString(typeExprString(d.Type.Results.List[0].Type))
		}
	}
	return b.String()
}

func paramsString(params *ast.FieldList) string {
	if params == nil || len(params.List) == 0 {
		return "()"
	}
	var parts []string
	for _, p := range params.List {
		names := fieldNames(p)
		typeStr := typeExprString(p.Type)
		if len(names) > 0 {
			for _, n := range names {
				parts = append(parts, fmt.Sprintf("%s %s", n, typeStr))
			}
		} else {
			parts = append(parts, typeStr)
		}
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func resultsString(results *ast.FieldList) string {
	if results == nil || len(results.List) == 0 {
		return ""
	}
	var parts []string
	for _, r := range results.List {
		typeStr := typeExprString(r.Type)
		parts = append(parts, typeStr)
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

func fieldNames(f *ast.Field) []string {
	if len(f.Names) == 0 {
		return nil
	}
	names := make([]string, len(f.Names))
	for i, n := range f.Names {
		names[i] = n.Name
	}
	return names
}

// typeSpecSignature returns a concise signature string for a type spec.
func typeSpecSignature(s *ast.TypeSpec) string {
	switch t := s.Type.(type) {
	case *ast.StructType:
		// Count fields
		n := 0
		if t.Fields != nil {
			n = len(t.Fields.List)
		}
		return fmt.Sprintf("struct{%d fields}", n)
	case *ast.InterfaceType:
		n := 0
		if t.Methods != nil {
			n = len(t.Methods.List)
		}
		return fmt.Sprintf("interface{%d methods}", n)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", typeExprString(t.Key), typeExprString(t.Value))
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeExprString(t.Elt)
		}
		return "[...]" + typeExprString(t.Elt)
	case *ast.FuncType:
		return "func" + paramsString(t.Params) + " " + resultsString(t.Results)
	default:
		return typeExprString(t)
	}
}

// parseAPI parses a directory and returns Go doc packages.
// Used internally; keeping the pattern for potential reuse.
func parseAPI(searchDir string, includeInternal bool) ([]*doc.Package, *token.FileSet, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, searchDir, func(info os.FileInfo) bool {
		if !strings.HasSuffix(info.Name(), ".go") {
			return false
		}
		if strings.HasSuffix(info.Name(), "_test.go") && !includeInternal {
			return false
		}
		return true
	}, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var result []*doc.Package
	for name, pkg := range pkgs {
		dp := doc.New(pkg, name, doc.AllDecls)
		result = append(result, dp)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, fset, nil
}

// firstLine returns the first line of a documentation string.
func firstLine(s string) string {
	if idx := strings.Index(s, "\n"); idx >= 0 {
		return s[:idx]
	}
	return s
}
