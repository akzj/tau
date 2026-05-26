package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// DiagResult holds workspace diagnostic information.
type DiagResult struct {
	FileTree      map[string]int `json:"file_tree"`      // extension → count
	GitBranch     string         `json:"git_branch"`
	GitDirty      bool           `json:"git_dirty"`
	LanguageStats map[string]int `json:"language_stats"` // language → file count
	TotalFiles    int            `json:"total_files"`
}

// WorkspaceDiagTool creates a workspace diagnostic tool.
func WorkspaceDiagTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"depth": {"type": "integer", "description": "Directory depth for file tree scan (1-3, default 2)"},
			"include_git": {"type": "boolean", "description": "Include git status information (default true)"}
		}
	}`)

	return core.Tool{
		Name:        "workspace_diag",
		Description: "Scan the workspace: file tree, git status, language statistics. Provides a snapshot of the project structure.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Depth      int  `json:"depth"`
				IncludeGit bool `json:"include_git"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Depth <= 0 || args.Depth > 3 {
				args.Depth = 2
			}
			// Default IncludeGit to true when not explicitly set
			// (bool zero value is false, so we check raw params)
			var rawMap map[string]any
			json.Unmarshal(raw, &rawMap)
			if _, ok := rawMap["include_git"]; !ok {
				args.IncludeGit = true
			}

			diag := scanWorkspace(args.Depth, args.IncludeGit)
			return formatDiag(diag)
		},
	}
}

func scanWorkspace(depth int, includeGit bool) DiagResult {
	r := DiagResult{
		FileTree:      make(map[string]int),
		LanguageStats: make(map[string]int),
	}

	// Scan file tree
	filepath.WalkDir(WorkspaceRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path == WorkspaceRoot {
				return nil
			}
			rel, _ := filepath.Rel(WorkspaceRoot, path)
			if strings.Count(rel, string(os.PathSeparator)) >= depth {
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == "" {
			ext = "(no ext)"
		}
		r.FileTree[ext]++
		r.TotalFiles++
		// Language classification
		switch ext {
		case ".go":
			r.LanguageStats["Go"]++
		case ".py", ".pyw":
			r.LanguageStats["Python"]++
		case ".js", ".ts", ".jsx", ".tsx":
			r.LanguageStats["JS/TS"]++
		case ".md", ".mdx":
			r.LanguageStats["Markdown"]++
		case ".json", ".yaml", ".yml", ".toml":
			r.LanguageStats["Config"]++
		default:
			r.LanguageStats["Other"]++
		}
		return nil
	})

	// Git status
	if includeGit {
		cmd := exec.Command("git", "-C", WorkspaceRoot, "rev-parse", "--abbrev-ref", "HEAD")
		out, err := cmd.Output()
		if err == nil {
			r.GitBranch = strings.TrimSpace(string(out))
		}
		cmd2 := exec.Command("git", "-C", WorkspaceRoot, "status", "--porcelain")
		out2, err2 := cmd2.Output()
		r.GitDirty = err2 == nil && len(out2) > 0
	}

	return r
}

func formatDiag(d DiagResult) (core.ToolResult, error) {
	var b strings.Builder
	b.WriteString("## Workspace Diagnostic\n\n")

	// Git info
	if d.GitBranch != "" {
		b.WriteString(fmt.Sprintf("**Branch**: %s", d.GitBranch))
		if d.GitDirty {
			b.WriteString(" (dirty)")
		}
		b.WriteString("\n\n")
	}

	// File tree
	b.WriteString(fmt.Sprintf("**Total files**: %d\n\n", d.TotalFiles))
	b.WriteString("### Language Breakdown\n")
	langs := []string{"Go", "Python", "JS/TS", "Markdown", "Config", "Other"}
	for _, lang := range langs {
		if count, ok := d.LanguageStats[lang]; ok && count > 0 {
			pct := float64(count) / float64(d.TotalFiles) * 100
			b.WriteString(fmt.Sprintf("- %s: %d files (%.1f%%)\n", lang, count, pct))
		}
	}

	// Extensions
	b.WriteString("\n### Extension Breakdown\n")
	type extCount struct {
		ext   string
		count int
	}
	var exts []extCount
	for ext, count := range d.FileTree {
		exts = append(exts, extCount{ext, count})
	}
	sort.Slice(exts, func(i, j int) bool { return exts[i].count > exts[j].count })
	for _, ec := range exts {
		b.WriteString(fmt.Sprintf("- %s: %d\n", ec.ext, ec.count))
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: b.String()}},
		Details: map[string]any{
			"total_files": d.TotalFiles,
			"git_branch":  d.GitBranch,
			"git_dirty":   d.GitDirty,
		},
	}, nil
}
