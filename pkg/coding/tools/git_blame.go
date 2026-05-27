package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// GitBlameTool creates a git blame tool for author tracing.
//
// Parameters:
//
//	file_path  (string, required) — file path relative to workspace root
//	line_start (int, optional) — start line (1-based)
//	line_end   (int, optional) — end line (1-based)
//
// Executes git blame -L to show author, commit, and time for each line.
func GitBlameTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"file_path": {"type": "string", "description": "File path relative to workspace root"},
			"line_start": {"type": "integer", "description": "Start line (1-based, optional)"},
			"line_end": {"type": "integer", "description": "End line (1-based, optional)"}
		},
		"required": ["file_path"]
	}`)

	return core.Tool{
		Name:        "git_blame",
		Description: "Git blame: trace line authorship. Shows author, commit, and timestamp per line.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FilePath  string `json:"file_path"`
				LineStart int    `json:"line_start"`
				LineEnd   int    `json:"line_end"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.FilePath == "" {
				return core.ToolResult{}, fmt.Errorf("file_path required")
			}

			resolved, err := ResolvePath(args.FilePath)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("resolve path: %w", err)
			}

			relPath, err := filepath.Rel(WorkspaceRoot, resolved)
			if err != nil {
				relPath = args.FilePath
			}

			cmdArgs := []string{"blame", "--date=short"}
			if args.LineStart > 0 {
				lineRange := fmt.Sprintf("%d", args.LineStart)
				if args.LineEnd > 0 {
					lineRange += fmt.Sprintf(",%d", args.LineEnd)
				}
				cmdArgs = append(cmdArgs, "-L", lineRange)
			}
			cmdArgs = append(cmdArgs, "--", relPath)

			timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(timeoutCtx, "git", cmdArgs...)
			cmd.Dir = WorkspaceRoot

			output, err := cmd.CombinedOutput()
			text := strings.TrimSpace(string(output))

			if err != nil {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("git blame failed:\n%s", text)}},
				}, err
			}

			if text == "" {
				text = fmt.Sprintf("(no blame info for %s)", relPath)
			}

			// Count authors
			authors := map[string]int{}
			for _, line := range strings.Split(text, "\n") {
				// git blame format: ^commit_hash (Author Date LineNum) content
				if idx := strings.Index(line, "("); idx >= 0 {
					rest := line[idx+1:]
					if spaceIdx := strings.Index(rest, " "); spaceIdx >= 0 {
						author := rest[:spaceIdx]
						authors[author]++
					}
				}
			}

			header := fmt.Sprintf("## Git Blame: %s\n\n", relPath)
			if len(authors) > 0 {
				header += "Authors: "
				var authorList []string
				for a := range authors {
					authorList = append(authorList, a)
				}
				header += strings.Join(authorList, ", ") + "\n\n"
			}
			header += "```\n" + text + "\n```"

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: header}},
				Details: map[string]any{"file": relPath, "authors": len(authors)},
			}, nil
		},
	}
}
