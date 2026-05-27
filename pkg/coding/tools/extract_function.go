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

// ExtractFunctionTool creates a tool for extracting code into a new function.
//
// Parameters:
//
//	file              (string, required) — file path relative to workspace root
//	start_line        (integer, required) — first line to extract (1-based)
//	end_line          (integer, required) — last line to extract (1-based)
//	new_function_name (string, required) — name of the new function
func ExtractFunctionTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file":              {"type": "string", "description": "File path relative to workspace root"},
			"start_line":        {"type": "integer", "description": "First line to extract (1-based)"},
			"end_line":          {"type": "integer", "description": "Last line to extract (1-based)"},
			"new_function_name": {"type": "string", "description": "Name of the new function"}
		},
		"required": ["file", "start_line", "end_line", "new_function_name"]
	}`)

	return core.Tool{
		Name:        "extract_function",
		Description: "Extract code lines into a new function. Replaces extracted lines with a call to the new function. Returns the diff summary.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File            string `json:"file"`
				StartLine       int    `json:"start_line"`
				EndLine         int    `json:"end_line"`
				NewFunctionName string `json:"new_function_name"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}
			if args.StartLine <= 0 || args.EndLine <= 0 {
				return core.ToolResult{}, fmt.Errorf("start_line and end_line must be > 0")
			}
			if args.StartLine >= args.EndLine {
				return core.ToolResult{}, fmt.Errorf("start_line must be less than end_line")
			}
			if args.NewFunctionName == "" {
				return core.ToolResult{}, fmt.Errorf("new_function_name required")
			}
			if !token.IsIdentifier(args.NewFunctionName) {
				return core.ToolResult{}, fmt.Errorf("new_function_name %q is not a valid Go identifier", args.NewFunctionName)
			}

			fullPath, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, err
			}

			content, err := os.ReadFile(fullPath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("read file: %w", err)
			}

			result, err := extractFunction(content, fullPath, args.StartLine, args.EndLine, args.NewFunctionName)
			if err != nil {
				return core.ToolResult{}, err
			}

			err = os.WriteFile(fullPath, []byte(result.NewContent), 0644)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("write file: %w", err)
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Extracted lines %d-%d into function %s():\n", args.StartLine, args.EndLine, args.NewFunctionName))
			output.WriteString(fmt.Sprintf("Lines extracted: %d\n", result.LinesExtracted))
			output.WriteString(fmt.Sprint("--- Extracted function ---\n"))
			output.WriteString(result.FunctionCode)
			output.WriteString("\n--- Call site replacement ---\n")
			output.WriteString(result.CallSite)

			resultText := output.String()
			if len(resultText) > OutputCap {
				resultText = resultText[:OutputCap] + "\n... (truncated)"
			}

			absPath, _ := filepath.Abs(fullPath)
			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: resultText}},
				Details: map[string]any{
					"file":            absPath,
					"start_line":      args.StartLine,
					"end_line":        args.EndLine,
					"new_function":    args.NewFunctionName,
					"lines_extracted": result.LinesExtracted,
				},
			}, nil
		},
	}
}

type extractResult struct {
	NewContent     string
	FunctionCode   string
	CallSite       string
	LinesExtracted int
}

func extractFunction(src []byte, fullPath string, startLine, endLine int, newFuncName string) (*extractResult, error) {
	lines := strings.Split(string(src), "\n")
	if endLine > len(lines) {
		return nil, fmt.Errorf("end_line %d exceeds file line count %d", endLine, len(lines))
	}

	// Extract the selected lines
	extractedLines := lines[startLine-1 : endLine]
	extractedCode := strings.Join(extractedLines, "\n")

	// Determine indentation of the first extracted line
	indent := detectIndent(extractedLines[0])

	// Build the new function
	funcIndent := indent
	var funcBody strings.Builder
	funcBody.WriteString("func " + newFuncName + "() {\n")
	for _, l := range extractedLines {
		funcBody.WriteString(l + "\n")
	}
	funcBody.WriteString("}")

	// Replace the extracted lines with a call
	callLine := fmt.Sprintf("%s%s()", indent, newFuncName)

	// Build new file content: lines before + call line + lines after + new function
	var newLines []string
	newLines = append(newLines, lines[:startLine-1]...)
	newLines = append(newLines, callLine)
	newLines = append(newLines, lines[endLine:]...)
	// Add the new function at the end
	newLines = append(newLines, "")
	newLines = append(newLines, funcBody.String())

	_ = funcIndent
	_ = extractedCode

	newContent := strings.Join(newLines, "\n")

	// Format the result
	fmtCode, err := format.Source([]byte(newContent))
	if err != nil {
		// Return unformatted content if formatting fails
		return &extractResult{
			NewContent:     newContent,
			FunctionCode:   funcBody.String(),
			CallSite:       callLine,
			LinesExtracted: endLine - startLine + 1,
		}, nil
	}

	return &extractResult{
		NewContent:     string(fmtCode),
		FunctionCode:   funcBody.String(),
		CallSite:       callLine,
		LinesExtracted: endLine - startLine + 1,
	}, nil
}

func detectIndent(line string) string {
	for i, c := range line {
		if c != '\t' && c != ' ' {
			return line[:i]
		}
	}
	return ""
}

// Ensure unused imports are available for compilation
var _ = ast.File{}    // for later phases
var _ = parser.ParseFile
var _ = format.Node
var _ = token.Pos(0)
