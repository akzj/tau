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
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// RefactorRenameTool creates a tool for safe symbol renaming.
//
// Parameters:
//
//	old_name (string, required) — current symbol name
//	new_name (string, required) — target symbol name
//	path     (string, optional) — file or directory path (default: workspace root)
//	dry_run  (boolean, optional) — preview only, no file changes (default: false)
func RefactorRenameTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"old_name": {"type": "string", "description": "Current symbol name to rename"},
			"new_name": {"type": "string", "description": "Target symbol name"},
			"path":     {"type": "string", "description": "File or directory path (default: workspace root)"},
			"dry_run":  {"type": "boolean", "description": "Preview only — don't write files (default: false)"}
		},
		"required": ["old_name", "new_name"]
	}`)

	return core.Tool{
		Name:        "refactor_rename",
		Description: "Safe symbol rename. Uses go/ast to find all references (declarations and usages) and apply rename. Returns files_changed and replacement count.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				OldName string `json:"old_name"`
				NewName string `json:"new_name"`
				Path    string `json:"path"`
				DryRun  bool   `json:"dry_run"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.OldName == "" {
				return core.ToolResult{}, fmt.Errorf("old_name required")
			}
			if args.NewName == "" {
				return core.ToolResult{}, fmt.Errorf("new_name required")
			}
			if !token.IsIdentifier(args.OldName) {
				return core.ToolResult{}, fmt.Errorf("old_name %q is not a valid Go identifier", args.OldName)
			}
			if !token.IsIdentifier(args.NewName) {
				return core.ToolResult{}, fmt.Errorf("new_name %q is not a valid Go identifier", args.NewName)
			}

			searchDir := WorkspaceRoot
			if args.Path != "" {
				var err error
				searchDir, err = ResolvePath(args.Path)
				if err != nil {
					return core.ToolResult{}, err
				}
			}

			changes, err := renameSymbol(searchDir, args.OldName, args.NewName, args.DryRun)
			if err != nil {
				return core.ToolResult{}, err
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Rename: %s → %s\n", args.OldName, args.NewName))
			if args.DryRun {
				output.WriteString("DRY RUN — no files modified.\n")
			}
			output.WriteString(fmt.Sprintf("Files changed: %d\nReplacements: %d\n", changes.FilesChanged, changes.Replacements))
			for _, c := range changes.FileDetails {
				output.WriteString(fmt.Sprintf("  %s: %d replacements\n", c.Path, c.Count))
			}

			result := output.String()
			if len(result) > OutputCap {
				result = result[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: result}},
				Details: map[string]any{
					"files_changed": changes.FilesChanged,
					"replacements":  changes.Replacements,
					"old_name":      args.OldName,
					"new_name":      args.NewName,
					"dry_run":       args.DryRun,
				},
			}, nil
		},
	}
}

type renameResult struct {
	FilesChanged int
	Replacements int
	FileDetails  []renameFileDetail
}

type renameFileDetail struct {
	Path  string
	Count int
}

func renameSymbol(root, oldName, newName string, dryRun bool) (*renameResult, error) {
	fset := token.NewFileSet()
	res := &renameResult{}

	// Collect all Go files
	var goFiles []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
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
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk: %w", err)
	}

	// Find the declaration position to identify if a usage is the decl itself
	declPositions := make(map[string]token.Pos) // filepath -> decl position

	// First pass: find declaration
	for _, fpath := range goFiles {
		file, err := parser.ParseFile(fset, fpath, nil, 0)
		if err != nil {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch nd := n.(type) {
			case *ast.FuncDecl:
				if nd.Name.Name == oldName {
					declPositions[fpath] = nd.Pos()
					return false
				}
			case *ast.GenDecl:
				for _, spec := range nd.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok {
						for _, ident := range vs.Names {
							if ident.Name == oldName {
								declPositions[fpath] = ident.Pos()
							}
						}
					}
					if ts, ok := spec.(*ast.TypeSpec); ok {
						if ts.Name.Name == oldName {
							declPositions[fpath] = ts.Pos()
						}
					}
				}
			}
			return true
		})
	}

	// Second pass: find all identifiers and replace
	for _, fpath := range goFiles {
		content, err := os.ReadFile(fpath)
		if err != nil {
			continue
		}
		file, err := parser.ParseFile(fset, fpath, content, parser.ParseComments)
		if err != nil {
			continue
		}

		// Collect all identifier positions that match oldName
		type replaceSite struct {
			pos token.Pos
		}
		var sites []replaceSite
		ast.Inspect(file, func(n ast.Node) bool {
			if ident, ok := n.(*ast.Ident); ok && ident.Name == oldName {
				sites = append(sites, replaceSite{pos: ident.Pos()})
			}
			return true
		})

		if len(sites) == 0 {
			continue
		}

		relPath, _ := filepath.Rel(root, fpath)
		count := 0

		if !dryRun {
			// Sort positions in descending order so replacements don't shift offsets
			// Actually we replace at positions. Let's rebuild the file.
			newContent := replaceIdentsInSource(fset, file, content, oldName, newName)
			if newContent != string(content) {
				err = os.WriteFile(fpath, []byte(newContent), 0644)
				if err != nil {
					return nil, fmt.Errorf("write %s: %w", fpath, err)
				}
				count = len(sites)
			}
		} else {
			count = len(sites)
		}

		if count > 0 {
			res.FilesChanged++
			res.Replacements += count
			res.FileDetails = append(res.FileDetails, renameFileDetail{
				Path:  relPath,
				Count: count,
			})
		}
	}

	return res, nil
}

// replaceIdentsInSource replaces all identifiers matching oldName with newName in the source.
// Uses the AST to find identifier positions and replaces them in descending offset order.
func replaceIdentsInSource(fset *token.FileSet, file *ast.File, src []byte, oldName, newName string) string {
	type posRange struct {
		start int
		end   int
	}

	var ranges []posRange
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == oldName {
			pos := fset.Position(ident.Pos())
			end := fset.Position(ident.End())
			ranges = append(ranges, posRange{start: pos.Offset, end: end.Offset})
		}
		return true
	})

	if len(ranges) == 0 {
		return string(src)
	}

	// Sort by offset descending
	for i := 0; i < len(ranges); i++ {
		for j := i + 1; j < len(ranges); j++ {
			if ranges[j].start > ranges[i].start {
				ranges[i], ranges[j] = ranges[j], ranges[i]
			}
		}
	}

	result := string(src)
	for _, r := range ranges {
		result = result[:r.start] + newName + result[r.end:]
	}

	// Validate: format the result to ensure it's valid Go
	fmtCode, err := format.Source([]byte(result))
	if err == nil {
		return string(fmtCode)
	}
	return result
}
