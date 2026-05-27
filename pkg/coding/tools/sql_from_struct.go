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

// SQLFromStructTool generates SQL CREATE TABLE DDL from a Go struct definition.
//
// Parameters:
//
//	file        (string, required) — Go source file containing the struct
//	struct_name (string, required) — name of the struct to convert
func SQLFromStructTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file": {"type": "string", "description": "Go source file containing the struct"},
			"struct_name": {"type": "string", "description": "Name of the struct to convert to SQL"}
		},
		"required": ["file", "struct_name"]
	}`)

	return core.Tool{
		Name:        "sql_from_struct",
		Description: "Generate SQL CREATE TABLE from a Go struct. Maps Go types to SQL types, detects json tags for column names.",
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

			resolved, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, resolved, nil, parser.ParseComments)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("parse Go file: %w", err)
			}

			// Find the struct
			var targetStruct *ast.StructType
			for _, decl := range f.Decls {
				genDecl, ok := decl.(*ast.GenDecl)
				if !ok || genDecl.Tok != token.TYPE {
					continue
				}
				for _, spec := range genDecl.Specs {
					typeSpec, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if typeSpec.Name.Name == args.StructName {
						st, ok := typeSpec.Type.(*ast.StructType)
						if ok {
							targetStruct = st
							break
						}
					}
				}
				if targetStruct != nil {
					break
				}
			}

			if targetStruct == nil {
				return core.ToolResult{}, fmt.Errorf("struct %q not found in %s", args.StructName, args.File)
			}

			tableName := toSnakeCase(args.StructName)
			ddl := buildCreateTable(tableName, targetStruct, fset)

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: ddl}},
				Details: map[string]any{
					"file":        args.File,
					"struct_name": args.StructName,
					"table_name":  tableName,
					"success":     true,
				},
			}, nil
		},
	}
}

// buildCreateTable generates CREATE TABLE DDL from a struct.
func buildCreateTable(tableName string, st *ast.StructType, fset *token.FileSet) string {
	var cols []string

	if st.Fields == nil || len(st.Fields.List) == 0 {
		return fmt.Sprintf("-- No fields in struct\nCREATE TABLE %s (\n  id INTEGER PRIMARY KEY\n);\n", tableName)
	}

	for _, field := range st.Fields.List {
		if field.Names == nil {
			// Embedded field — skip for simplicity
			continue
		}

		for _, name := range field.Names {
			colName := toSnakeCase(name.Name)
			colType := goTypeToSQL(exprToString(field.Type))

			// Check for json tag
			if field.Tag != nil {
				tag := strings.Trim(field.Tag.Value, "`")
				jsonTag := extractJSONTag(tag)
				if jsonTag != "" && jsonTag != "-" {
					colName = jsonTag
				}
			}

			col := fmt.Sprintf("  %s %s", colName, colType)
			cols = append(cols, col)
		}
	}

	ddl := fmt.Sprintf("-- Generated from Go struct\nCREATE TABLE IF NOT EXISTS %s (\n%s\n);\n", tableName, strings.Join(cols, ",\n"))
	return ddl
}

// goTypeToSQL maps Go types to SQL column types.
func goTypeToSQL(goType string) string {
	// Strip pointer prefix
	goType = strings.TrimPrefix(goType, "*")
	goType = strings.TrimSpace(goType)

	switch goType {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"byte", "rune":
		return "INTEGER"
	case "float32", "float64":
		return "REAL"
	case "bool":
		return "BOOLEAN"
	case "string":
		return "TEXT"
	case "time.Time":
		return "TIMESTAMP"
	default:
		// Check for custom types — default to TEXT
		if strings.Contains(goType, ".") {
			// Qualified type like sql.NullString
			if strings.HasSuffix(goType, "NullString") {
				return "TEXT"
			}
			if strings.HasSuffix(goType, "NullInt64") || strings.HasSuffix(goType, "NullInt32") {
				return "INTEGER"
			}
			if strings.HasSuffix(goType, "NullFloat64") {
				return "REAL"
			}
			if strings.HasSuffix(goType, "NullBool") {
				return "BOOLEAN"
			}
			if strings.HasSuffix(goType, "NullTime") {
				return "TIMESTAMP"
			}
		}
		return "TEXT"
	}
}

// exprToString converts an AST expression to its string representation.
func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		if e.Len == nil {
			return "[]" + exprToString(e.Elt)
		}
		return fmt.Sprintf("[%s]%s", exprToString(e.Len), exprToString(e.Elt))
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToString(e.Key), exprToString(e.Value))
	default:
		return "interface{}"
	}
}

// extractJSONTag extracts the json tag name from a struct field tag.
func extractJSONTag(tag string) string {
	st := reflectTagLookup(tag, "json")
	if st == "" {
		return ""
	}
	// Split on comma: "name,omitempty"
	parts := strings.Split(st, ",")
	if len(parts) > 0 && parts[0] != "" {
		return parts[0]
	}
	return ""
}

// reflectTagLookup is a simplified version of reflect.StructTag.Lookup.
func reflectTagLookup(tag, key string) string {
	for tag != "" {
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		i = 0
		for i < len(tag) && tag[i] != ':' && tag[i] != ' ' {
			i++
		}
		if i == 0 || i >= len(tag) || tag[i] != ':' {
			break
		}
		if tag[:i] == key {
			tag = tag[i+1:]
			if tag == "" {
				return ""
			}
			if tag[0] == '"' {
				tag = tag[1:]
				j := 0
				for j < len(tag) && tag[j] != '"' {
					if tag[j] == '\\' {
						j++
					}
					j++
				}
				return tag[:j]
			}
			j := 0
			for j < len(tag) && tag[j] != ' ' {
				j++
			}
			return tag[:j]
		}
		// Skip to next key
		i = 0
		for i < len(tag) && tag[i] != ' ' {
			if tag[i] == '"' {
				i++
				for i < len(tag) && tag[i] != '"' {
					if tag[i] == '\\' {
						i++
					}
					i++
				}
			}
			i++
		}
		tag = tag[i:]
	}
	return ""
}

// toSnakeCase converts CamelCase to snake_case.
// Handles acronyms: "ItemID" → "item_id", "HTTPServer" → "http_server".
func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			c = c + 32 // to lower
			// Insert underscore before uppercase unless:
			// - it's the first character
			// - the previous character is already uppercase AND the next is lowercase
			//   (end of an acronym: "ID" in "ItemID" or "HTTP" in "HTTPServer")
			if i > 0 {
				prev := s[i-1]
				if prev >= 'a' && prev <= 'z' {
					// Lowercase → uppercase transition: always insert _
					result = append(result, '_')
				} else if i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
					// Uppercase → uppercase → lowercase: end of acronym
					result = append(result, '_')
				}
			}
			result = append(result, byte(c))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
