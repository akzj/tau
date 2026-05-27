package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// LicenseHeaderTool creates a license header checker/fixer.
//
// Parameters:
//
//	path                (string, required) — directory or file path to scan
//	license_header_text (string, required) — the expected license header text
//	action              (string, optional) — check or fix (default: check)
func LicenseHeaderTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory or file path to scan"},
			"license_header_text": {"type": "string", "description": "The expected license header text to check/add"},
			"action": {"type": "string", "description": "check or fix (default: check)"}
		},
		"required": ["path", "license_header_text"]
	}`)

	return core.Tool{
		Name:        "license_header",
		Description: "License header checker/fixer. Scans source files for license headers, adds missing headers, fixes incorrect ones. Returns files_checked, missing, fixed.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path               string `json:"path"`
				LicenseHeaderText   string `json:"license_header_text"`
				Action             string `json:"action"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.LicenseHeaderText == "" {
				return core.ToolResult{}, fmt.Errorf("license_header_text required")
			}
			if args.Action == "" {
				args.Action = "check"
			}

			scanDir, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			// Normalize header text: trim whitespace and normalize newlines
			expected := normalizeHeader(args.LicenseHeaderText)

			result := processLicenseHeaders(scanDir, expected, args.Action, onUpdate)

			return buildLicenseHeaderResult(result), nil
		},
	}
}

type licenseHeaderResult struct {
	FilesChecked int      `json:"files_checked"`
	Missing      int      `json:"missing"`
	Fixed        int      `json:"fixed"`
	Correct      int      `json:"correct"`
	Failures     []string `json:"failures,omitempty"`
}

var sourceExtensions = map[string]bool{
	".go": true, ".py": true, ".js": true, ".ts": true, ".tsx": true, ".jsx": true,
	".java": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true, ".cc": true,
	".rs": true, ".rb": true, ".php": true, ".swift": true, ".kt": true, ".scala": true,
	".cs": true, ".sh": true, ".bash": true, ".zsh": true,
	".yaml": true, ".yml": true, ".toml": true,
}

var licenseSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, ".svn": true,
	"testdata": true, "__pycache__": true, "dist": true,
}

func normalizeHeader(header string) string {
	// Normalize line endings and trim
	header = strings.ReplaceAll(header, "\r\n", "\n")
	header = strings.TrimSpace(header)
	return header
}

func processLicenseHeaders(root, expected, action string, onUpdate func(core.PartialResult)) licenseHeaderResult {
	var result licenseHeaderResult

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if licenseSkipDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !sourceExtensions[ext] {
			return nil
		}

		result.FilesChecked++

		data, err := os.ReadFile(path)
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("%s: read error: %v", path, err))
			return nil
		}
		content := string(data)

		// Check if header is present (first N bytes)
		hasHeader := hasLicenseHeader(content, expected)

		if hasHeader {
			result.Correct++
			return nil
		}

		// Missing or incorrect
		result.Missing++

		if action == "fix" {
			if fixLicenseHeader(path, expected, content, ext) {
				result.Fixed++
			} else {
				result.Failures = append(result.Failures, fmt.Sprintf("%s: fix failed", path))
			}
		}

		return nil
	})

	return result
}

func hasLicenseHeader(content, expected string) bool {
	normalized := normalizeHeader(content)

	// Skip shebang if present
	searchStart := 0
	if strings.HasPrefix(normalized, "#!") {
		idx := strings.Index(normalized, "\n")
		if idx > 0 {
			searchStart = idx + 1
		}
	}

	// Strip comment prefixes from the file content for comparison
	body := normalized[searchStart:]
	bodyLines := strings.Split(body, "\n")
	var strippedLines []string
	for _, line := range bodyLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" && len(strippedLines) == 0 {
			continue // skip leading blank lines
		}
		// Strip common comment prefixes
		for _, prefix := range []string{"// ", "//", "# ", "#", "/* ", " * ", " */"} {
			if strings.HasPrefix(trimmed, prefix) {
				trimmed = strings.TrimPrefix(trimmed, prefix)
				break
			}
		}
		strippedLines = append(strippedLines, trimmed)
	}

	stripped := strings.TrimSpace(strings.Join(strippedLines, "\n"))

	// Check if expected text appears at the start of the stripped content
	expectedNorm := normalizeHeader(expected)
	return strings.HasPrefix(stripped, expectedNorm)
}

func fixLicenseHeader(path, expected, content, ext string) bool {
	// Determine comment style
	commentPrefix := getCommentPrefix(ext)
	if commentPrefix == "" {
		return false
	}

	// Build header with comment prefix
	headerLines := strings.Split(expected, "\n")
	var commentedHeader string
	for _, line := range headerLines {
		if strings.TrimSpace(line) == "" {
			commentedHeader += commentPrefix + "\n"
		} else {
			commentedHeader += commentPrefix + " " + line + "\n"
		}
	}

	// If file has shebang, keep it first
	var newContent string
	normalized := normalizeHeader(content)
	if strings.HasPrefix(normalized, "#!") {
		idx := strings.Index(normalized, "\n")
		shebang := normalized[:idx+1]
		rest := normalized[idx+1:]
		rest = strings.TrimLeft(rest, "\n")
		newContent = shebang + "\n" + commentedHeader + "\n" + rest
	} else {
		// Remove any existing header-style comments at start
		body := stripExistingHeaderComments(normalized, commentPrefix)
		newContent = commentedHeader + "\n" + body
	}

	if err := os.WriteFile(path, []byte(newContent), 0644); err != nil {
		return false
	}
	return true
}

func getCommentPrefix(ext string) string {
	switch ext {
	case ".go", ".java", ".c", ".h", ".cpp", ".cc", ".hpp", ".rs", ".swift", ".kt", ".scala", ".cs", ".js", ".ts", ".jsx", ".tsx", ".php":
		return "//"
	case ".py", ".rb", ".sh", ".bash", ".zsh", ".yaml", ".yml", ".toml":
		return "#"
	default:
		return ""
	}
}

func stripExistingHeaderComments(content, commentPrefix string) string {
	lines := strings.Split(content, "\n")
	// Skip leading blank lines and comment lines
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}
		if strings.HasPrefix(trimmed, commentPrefix) {
			i++
			continue
		}
		break
	}
	return strings.Join(lines[i:], "\n")
}

func buildLicenseHeaderResult(result licenseHeaderResult) core.ToolResult {
	var lines []string
	lines = append(lines, fmt.Sprintf("Files checked: %d", result.FilesChecked))
	lines = append(lines, fmt.Sprintf("  Correct: %d", result.Correct))
	lines = append(lines, fmt.Sprintf("  Missing: %d", result.Missing))
	lines = append(lines, fmt.Sprintf("  Fixed: %d", result.Fixed))

	if len(result.Failures) > 0 {
		lines = append(lines, "\nFailures:")
		for _, f := range result.Failures {
			lines = append(lines, "  "+f)
		}
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"files_checked": result.FilesChecked,
			"missing":       result.Missing,
			"fixed":         result.Fixed,
			"correct":       result.Correct,
			"success":       result.Missing == 0 || result.Fixed == result.Missing,
			"failures":      result.Failures,
		},
	}
}
