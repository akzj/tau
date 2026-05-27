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
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// AddErrorWrappingTool creates a tool that adds error wrapping to bare error returns.
//
// Parameters:
//
//	file    (string, required) — file path relative to workspace root
//	pattern (string, optional) — regex pattern matching error return statements to wrap
//	                               (default: all bare "return err" patterns)
func AddErrorWrappingTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file":    {"type": "string", "description": "File path relative to workspace root"},
			"pattern": {"type": "string", "description": "Regex pattern to match error return statements (default: all bare 'return err')"}
		},
		"required": ["file"]
	}`)

	return core.Tool{
		Name:        "add_error_wrapping",
		Description: "Add error wrapping. Wraps bare `return err` or matched patterns with `fmt.Errorf(\"context: %w\", err)`. Returns files_changed and wraps_added count.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				File    string `json:"file"`
				Pattern string `json:"pattern"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.File == "" {
				return core.ToolResult{}, fmt.Errorf("file required")
			}

			fullPath, err := ResolvePath(args.File)
			if err != nil {
				return core.ToolResult{}, err
			}

			var matchRe *regexp.Regexp
			if args.Pattern != "" {
				matchRe, err = regexp.Compile(args.Pattern)
				if err != nil {
					return core.ToolResult{}, fmt.Errorf("invalid regex pattern: %w", err)
				}
			}

			result, err := addErrorWrapping(fullPath, matchRe)
			if err != nil {
				return core.ToolResult{}, err
			}

			err = os.WriteFile(fullPath, []byte(result.NewContent), 0644)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("write file: %w", err)
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("Error wrapping: %s\n", args.File))
			output.WriteString(fmt.Sprintf("Wraps added: %d\n", result.WrapsAdded))
			for _, w := range result.WrapDetails {
				output.WriteString(fmt.Sprintf("  Line %d: %s\n", w.Line, w.Context))
			}

			outputText := output.String()
			if len(outputText) > OutputCap {
				outputText = outputText[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: outputText}},
				Details: map[string]any{
					"file":        fullPath,
					"wraps_added": result.WrapsAdded,
				},
			}, nil
		},
	}
}

type wrapResult struct {
	NewContent  string
	WrapsAdded  int
	WrapDetails []wrapDetail
}

type wrapDetail struct {
	Line    int
	Context string
}

func addErrorWrapping(fullPath string, matchRe *regexp.Regexp) (*wrapResult, error) {
	fset := token.NewFileSet()
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	file, err := parser.ParseFile(fset, fullPath, content, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	// Find all return statements that return a bare error variable
	// Pattern: `return err` where err is a named error value
	type replaceSite struct {
		start   int
		end     int
		line    int
		context string
		newText string
	}

	var sites []replaceSite

	ast.Inspect(file, func(n ast.Node) bool {
		retStmt, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Check if this is a statement like `return err`
		if len(retStmt.Results) == 0 {
			return true
		}

		pos := fset.Position(retStmt.Pos())
		end := fset.Position(retStmt.End())

		// Get the original text of this return statement
		if pos.Offset < 0 || end.Offset < 0 || end.Offset > len(content) {
			return true
		}
		origText := string(content[pos.Offset:end.Offset])

		// If a pattern is provided, check if this return matches
		if matchRe != nil {
			if !matchRe.MatchString(origText) {
				return true
			}
		} else {
			// Default: only wrap bare `return err` (single error result)
			if !isBareErrorReturn(retStmt, origText) {
				return true
			}
		}

		// Determine the function context for the error message
		funcName := findEnclosingFunction(file, retStmt.Pos())
		contextMsg := funcName
		if contextMsg == "" {
			contextMsg = "operation"
		}

		// Build the replacement: `return fmt.Errorf("contextMsg: %w", err)`
		newText := buildWrappedReturn(retStmt, origText, contextMsg, content, fset)

		sites = append(sites, replaceSite{
			start:   pos.Offset,
			end:     end.Offset,
			line:    pos.Line,
			context: contextMsg,
			newText: newText,
		})
		return true
	})

	if len(sites) == 0 {
		return &wrapResult{
			NewContent: string(content),
			WrapsAdded: 0,
		}, nil
	}

	// Apply replacements in descending offset order
	for i := 0; i < len(sites); i++ {
		for j := i + 1; j < len(sites); j++ {
			if sites[j].start > sites[i].start {
				sites[i], sites[j] = sites[j], sites[i]
			}
		}
	}

	result := string(content)
	var details []wrapDetail
	for _, s := range sites {
		result = result[:s.start] + s.newText + result[s.end:]
		details = append(details, wrapDetail{
			Line:    s.line,
			Context: s.context,
		})
	}

	// Check if fmt is already imported; if not, we need to add it
	if strings.Count(result, `"fmt"`) == 0 && strings.Count(result, "fmt.Errorf") > 0 {
		result = addFmtImport(result)
	}

	// Format
	fmtCode, err := format.Source([]byte(result))
	if err != nil {
		return &wrapResult{
			NewContent:  result,
			WrapsAdded:  len(sites),
			WrapDetails: details,
		}, nil
	}

	return &wrapResult{
		NewContent:  string(fmtCode),
		WrapsAdded:  len(sites),
		WrapDetails: details,
	}, nil
}

func isBareErrorReturn(ret *ast.ReturnStmt, text string) bool {
	// Must have exactly 1 result that is a bare identifier
	if len(ret.Results) != 1 {
		return false
	}
	ident, ok := ret.Results[0].(*ast.Ident)
	if !ok {
		return false
	}
	// The text should match `return <name>` — simple pattern
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "return ") &&
		!strings.Contains(trimmed, ",") &&
		!strings.Contains(trimmed, "(") &&
		ident.Name != "nil"
}

func findEnclosingFunction(file *ast.File, pos token.Pos) string {
	var funcName string
	ast.Inspect(file, func(n ast.Node) bool {
		if fd, ok := n.(*ast.FuncDecl); ok {
			if fd.Body != nil {
				fdPos := file.FileStart + token.Pos(file.Pos()) // approximate
				_ = fdPos
				start := fd.Pos()
				end := fd.End()
				if pos >= start && pos <= end {
					funcName = fd.Name.Name
					if fd.Recv != nil && len(fd.Recv.List) > 0 {
						recvType := extractReceiverType(fd.Recv.List[0].Type)
						if recvType != "" {
							funcName = recvType + "." + funcName
						}
					}
					return false
				}
			}
		}
		return true
	})
	return funcName
}

func buildWrappedReturn(ret *ast.ReturnStmt, origText, contextMsg string, content []byte, fset *token.FileSet) string {
	// Extract the error variable name
	var errName string
	for _, r := range ret.Results {
		if ident, ok := r.(*ast.Ident); ok {
			errName = ident.Name
		}
	}
	if errName == "" {
		return origText
	}

	// Build: return fmt.Errorf("contextMsg: %w", errName)
	indent := detectLineIndent(origText)
	return fmt.Sprintf(`%sreturn fmt.Errorf("%s: %%w", %s)`, indent, contextMsg, errName)
}

func detectLineIndent(line string) string {
	for i, c := range line {
		if c != '\t' && c != ' ' {
			return line[:i]
		}
	}
	return ""
}

func addFmtImport(src string) string {
	// Determine indentation from the existing file
	indent := "\t"
	if strings.Contains(src, "    ") {
		indent = "    "
	}

	// Check for existing import block
	importRe := regexp.MustCompile(`(?s)import\s*\(([^)]*)\)`)
	loc := importRe.FindStringIndex(src)
	if loc != nil {
		// Already has import block — check if "fmt" is inside
		inner := src[loc[0]:loc[1]]
		if strings.Contains(inner, `"fmt"`) {
			return src
		}
		// Insert "fmt" at the start of the block
		before := src[:loc[0]]
		blockStart := strings.Index(inner, "(") + 1
		newBlock := inner[:blockStart] + "\n" + indent + `"fmt"` + inner[blockStart:]
		return before + newBlock + src[loc[1]:]
	}

	// Check for single import
	singleRe := regexp.MustCompile(`import\s+"[^"]+"`)
	singleLoc := singleRe.FindStringIndex(src)
	if singleLoc != nil {
		// Convert single import to import block with fmt added
		single := src[singleLoc[0]:singleLoc[1]]
		newBlock := "import (\n" + indent + `"fmt"` + "\n" + indent + single[7:] + "\n)"
		return src[:singleLoc[0]] + newBlock + src[singleLoc[1]:]
	}

	// No imports at all — add after package declaration
	packageRe := regexp.MustCompile(`package\s+\w+\s*\n`)
	pkgLoc := packageRe.FindStringIndex(src)
	if pkgLoc != nil {
		return src[:pkgLoc[1]] + "\nimport \"fmt\"\n\n" + src[pkgLoc[1]:]
	}

	return src
}
