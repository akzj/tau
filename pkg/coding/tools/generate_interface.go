package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"

	"github.com/akzj/tau/core"
)

// GenerateInterfaceTool creates a tool that generates an interface from a struct.
//
// Parameters:
//
//	file        (string, required) — file path relative to workspace root
//	struct_name (string, required) — struct name to extract interface from
func GenerateInterfaceTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file":        {"type": "string", "description": "File path relative to workspace root"},
			"struct_name": {"type": "string", "description": "Struct name to extract interface from"}
		},
		"required": ["file", "struct_name"]
	}`)

	return core.Tool{
		Name:        "generate_interface",
		Description: "Generate an interface from a struct. Extracts exported methods and creates an interface definition. Returns the interface code.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File       string `json:"file"`
				StructName string `json:"struct_name"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}
			if args.StructName == "" {
				return core.ToolResult{}, fmt.Errorf("struct_name required")
			}

			fullPath, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, err
			}

			iface, err := generateInterface(fullPath, args.StructName)
			if err != nil {
				return core.ToolResult{}, err
			}

			result := iface
			if len(result) > OutputCap {
				result = result[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: result}},
				Details: map[string]any{
					"file":         fullPath,
					"struct_name":  args.StructName,
					"method_count": strings.Count(iface, "\n") - 2, // rough estimate
				},
			}, nil
		},
	}
}

// generateInterface parses a Go file, finds the struct, and generates an interface
// with all its exported method signatures.
func generateInterface(filePath, structName string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse file: %w", err)
	}

	// Collect exported methods from the struct's method receivers
	type methodSig struct {
		name string
		sig  string
	}

	var methods []methodSig

	// Check for inline receivers (*Struct or Struct)
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || len(fd.Recv.List) == 0 {
			continue
		}

		recvType := fd.Recv.List[0].Type
		typeName := extractReceiverType(recvType)
		if typeName != structName {
			continue
		}

		// Only exported methods
		if !fd.Name.IsExported() {
			continue
		}

		sig := extractMethodSignature(fset, fd)
		methods = append(methods, methodSig{
			name: fd.Name.Name,
			sig:  sig,
		})
	}

	if len(methods) == 0 {
		return "", fmt.Errorf("struct %q not found or has no exported methods", structName)
	}

	// Generate interface
	ifaceName := structName
	if strings.HasPrefix(ifaceName, "I") {
		// Keep as-is, it might already be interface-like
	} else {
		ifaceName = "I" + ifaceName
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("// %s is an interface for %s methods.\n", ifaceName, structName))
	output.WriteString(fmt.Sprintf("type %s interface {\n", ifaceName))
	for _, m := range methods {
		output.WriteString(fmt.Sprintf("\t%s\n", m.sig))
	}
	output.WriteString("}\n")

	return output.String(), nil
}

// extractReceiverType gets the struct name from a receiver type expression.
// Handles *Struct and Struct forms.
func extractReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

// extractMethodSignature extracts the method signature without the body.
func extractMethodSignature(fset *token.FileSet, fd *ast.FuncDecl) string {
	var parts []string
	parts = append(parts, fd.Name.Name)

	// Parameters
	parts = append(parts, "(")
	for i, p := range fd.Type.Params.List {
		if i > 0 {
			parts = append(parts, ", ")
		}
		names := make([]string, len(p.Names))
		for j, n := range p.Names {
			names[j] = n.Name
		}
		parts = append(parts, strings.Join(names, ", "))
		if len(p.Names) > 0 {
			parts = append(parts, " ")
		}
		parts = append(parts, exprTypeString(p.Type))
	}
	parts = append(parts, ")")

	// Return type
	if fd.Type.Results != nil && len(fd.Type.Results.List) > 0 {
		parts = append(parts, " ")
		if len(fd.Type.Results.List) == 1 && fd.Type.Results.List[0].Names == nil {
			parts = append(parts, exprTypeString(fd.Type.Results.List[0].Type))
		} else {
			parts = append(parts, "(")
			for i, r := range fd.Type.Results.List {
				if i > 0 {
					parts = append(parts, ", ")
				}
				names := make([]string, len(r.Names))
				for j, n := range r.Names {
					names[j] = n.Name
				}
				parts = append(parts, strings.Join(names, ", "))
				if len(r.Names) > 0 {
					parts = append(parts, " ")
				}
				parts = append(parts, exprTypeString(r.Type))
			}
			parts = append(parts, ")")
		}
	}

	return strings.Join(parts, "")
}

// exprTypeString converts an ast.Expr for a type to a string representation.
func exprTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprTypeString(t.X)
	case *ast.SelectorExpr:
		return exprTypeString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprTypeString(t.Elt)
		}
		return "[...]" + exprTypeString(t.Elt) // approximate
	case *ast.MapType:
		return "map[" + exprTypeString(t.Key) + "]" + exprTypeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + exprTypeString(t.Value)
		case ast.RECV:
			return "<-chan " + exprTypeString(t.Value)
		default:
			return "chan " + exprTypeString(t.Value)
		}
	case *ast.FuncType:
		return "func" + funcTypeParams(t)
	case *ast.Ellipsis:
		return "..." + exprTypeString(t.Elt)
	case *ast.StructType:
		return "struct{...}"
	default:
		return "interface{}"
	}
}

func funcTypeParams(ft *ast.FuncType) string {
	var parts []string
	parts = append(parts, "(")
	for i, p := range ft.Params.List {
		if i > 0 {
			parts = append(parts, ", ")
		}
		parts = append(parts, exprTypeString(p.Type))
	}
	parts = append(parts, ")")
	if ft.Results != nil && len(ft.Results.List) > 0 {
		parts = append(parts, " ")
		if len(ft.Results.List) == 1 {
			parts = append(parts, exprTypeString(ft.Results.List[0].Type))
		} else {
			parts = append(parts, "(")
			for i, r := range ft.Results.List {
				if i > 0 {
					parts = append(parts, ", ")
				}
				parts = append(parts, exprTypeString(r.Type))
			}
			parts = append(parts, ")")
		}
	}
	return strings.Join(parts, "")
}
