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

// GenerateMocksTool creates a tool that generates mock implementations from interfaces.
//
// Parameters:
//
//	file           (string, required) — file path relative to workspace root
//	interface_name (string, required) — interface name to mock
//	package_name   (string, optional) — package name for generated mock (default: file's package)
func GenerateMocksTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file":           {"type": "string", "description": "File path relative to workspace root"},
			"interface_name": {"type": "string", "description": "Interface name to generate mock for"},
			"package_name":   {"type": "string", "description": "Package name for generated mock (default: file's package)"}
		},
		"required": ["file", "interface_name"]
	}`)

	return core.Tool{
		Name:        "generate_mocks",
		Description: "Generate a test mock implementation from an interface. Uses a struct with function fields for each method. Returns the mock code.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File          string `json:"file"`
				InterfaceName string `json:"interface_name"`
				PackageName   string `json:"package_name"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}
			if args.InterfaceName == "" {
				return core.ToolResult{}, fmt.Errorf("interface_name required")
			}

			fullPath, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, err
			}

			mockCode, err := generateMocks(fullPath, args.InterfaceName, args.PackageName)
			if err != nil {
				return core.ToolResult{}, err
			}

			if len(mockCode) > OutputCap {
				mockCode = mockCode[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: mockCode}},
				Details: map[string]any{
					"file":           fullPath,
					"interface_name": args.InterfaceName,
					"package_name":   args.PackageName,
				},
			}, nil
		},
	}
}

// generateMocks parses the Go file, finds the interface, and generates a mock struct.
func generateMocks(filePath, ifaceName, pkgName string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return "", fmt.Errorf("parse file: %w", err)
	}

	// Find the interface
	var ifaceType *ast.InterfaceType
	var foundPkg string
	foundPkg = file.Name.Name

	ast.Inspect(file, func(n ast.Node) bool {
		if ifaceType != nil {
			return false
		}
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if ts.Name.Name == ifaceName {
			if it, ok := ts.Type.(*ast.InterfaceType); ok {
				ifaceType = it
				return false
			}
		}
		return true
	})

	if ifaceType == nil {
		return "", fmt.Errorf("interface %q not found in file", ifaceName)
	}

	if pkgName == "" {
		pkgName = foundPkg
	}

	// Collect method signatures
	type methodInfo struct {
		name    string
		params  string
		results string
	}

	var methods []methodInfo
	for _, m := range ifaceType.Methods.List {
		if len(m.Names) == 0 {
			// Embedded interface — skip
			continue
		}
		for _, name := range m.Names {
			if ft, ok := m.Type.(*ast.FuncType); ok {
				methods = append(methods, methodInfo{
					name:    name.Name,
					params:  funcTypeParamsString(ft, true),
					results: funcTypeResultsString(ft),
				})
			}
		}
	}

	if len(methods) == 0 {
		return "", fmt.Errorf("interface %q has no methods", ifaceName)
	}

	// Generate mock struct
	mockStructName := "Mock" + ifaceName

	var output strings.Builder
	output.WriteString(fmt.Sprintf("// %s is a mock implementation of %s.\n", mockStructName, ifaceName))
	output.WriteString(fmt.Sprintf("type %s struct {\n", mockStructName))
	for _, m := range methods {
		// Generate function field type
		funcType := fmt.Sprintf("func(%s) %s", m.params, m.results)
		output.WriteString(fmt.Sprintf("\t%sFunc %s\n", m.name, funcType))
	}
	output.WriteString("}\n\n")

	// Generate method implementations
	for _, m := range methods {
		output.WriteString(fmt.Sprintf("func (m *%s) %s(%s) %s {\n", mockStructName, m.name, m.params, m.results))
		output.WriteString(fmt.Sprintf("\tif m.%sFunc != nil {\n", m.name))
		// Build call to the function field
		// We need parameter names — extract them from the signature string
		paramNames := extractParamNames(m.params)
		callArgs := strings.Join(paramNames, ", ")
		output.WriteString(fmt.Sprintf("\t\treturn m.%sFunc(%s)\n", m.name, callArgs))
		output.WriteString("\t}\n")
		// Return zero values
		if m.results != "" {
			zeroVals := zeroValues(m.results)
			output.WriteString(fmt.Sprintf("\treturn %s\n", zeroVals))
		}
		output.WriteString("}\n\n")
	}

	return output.String(), nil
}

// funcTypeParamsString returns the parameter list as a string.
func funcTypeParamsString(ft *ast.FuncType, withNames bool) string {
	var parts []string
	for _, p := range ft.Params.List {
		names := make([]string, len(p.Names))
		for j, n := range p.Names {
			names[j] = n.Name
		}
		typeStr := typeExprString(p.Type)
		if withNames && len(names) > 0 {
			parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
		} else if withNames {
			parts = append(parts, typeStr)
		} else {
			parts = append(parts, typeStr)
		}
	}
	return strings.Join(parts, ", ")
}

// funcTypeResultsString returns the result list as a string.
func funcTypeResultsString(ft *ast.FuncType) string {
	if ft.Results == nil || len(ft.Results.List) == 0 {
		return ""
	}
	var parts []string
	for _, r := range ft.Results.List {
		typeStr := typeExprString(r.Type)
		if len(r.Names) > 0 {
			names := make([]string, len(r.Names))
			for j, n := range r.Names {
				names[j] = n.Name
			}
			parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
		} else {
			parts = append(parts, typeStr)
		}
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// extractParamNames extracts parameter names from a function parameter string.
func extractParamNames(params string) []string {
	if params == "" {
		return nil
	}
	var names []string
	for _, p := range strings.Split(params, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts := strings.Fields(p)
		if len(parts) >= 2 {
			// last part is type, rest are names
			typeIdx := len(parts) - 1
			for i := 0; i < typeIdx; i++ {
				names = append(names, parts[i])
			}
		} else {
			// just a type, parameter is unnamed
			names = append(names, "_")
		}
	}
	return names
}

// zeroValues returns a comma-separated list of zero values for the given results.
func zeroValues(results string) string {
	var parts []string
	for _, r := range strings.Split(results, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		// Check if wrapped in parens
		if strings.HasPrefix(r, "(") && strings.HasSuffix(r, ")") {
			inner := r[1 : len(r)-1]
			return zeroValues(inner)
		}
		parts = append(parts, zeroForType(strings.TrimSpace(r)))
	}
	return strings.Join(parts, ", ")
}

// zeroForType returns a zero value for a Go type string.
func zeroForType(typeStr string) string {
	// Remove parameter names (take last word)
	words := strings.Fields(typeStr)
	if len(words) > 0 {
		typeStr = words[len(words)-1]
	}

	switch {
	case typeStr == "string":
		return `""`
	case typeStr == "bool":
		return "false"
	case strings.HasPrefix(typeStr, "int") ||
		strings.HasPrefix(typeStr, "uint") ||
		strings.HasPrefix(typeStr, "float") ||
		typeStr == "byte" || typeStr == "rune":
		return "0"
	case typeStr == "error":
		return "nil"
	case strings.HasPrefix(typeStr, "[]") ||
		strings.HasPrefix(typeStr, "map[") ||
		strings.HasPrefix(typeStr, "*") ||
		strings.HasPrefix(typeStr, "func"):
		return "nil"
	case strings.HasPrefix(typeStr, "chan"):
		return "nil"
	case strings.HasPrefix(typeStr, "interface"):
		return "nil"
	default:
		return "nil"
	}
}
