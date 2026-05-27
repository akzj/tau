package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// ReleaseNotesTool creates a release notes generation tool.
// Parameters: from_tag, to_tag, format (markdown/json).
// Parses git log between tags, categorizes commits (feat/fix/chore/docs).
// Returns categorized changelog.
func ReleaseNotesTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"from_tag": {"type": "string", "description": "Starting tag (e.g., v1.0.0)"},
			"to_tag": {"type": "string", "description": "Ending tag (e.g., v1.1.0, default: HEAD)"},
			"format": {"type": "string", "description": "Output format: markdown, json (default: markdown)"}
		},
		"required": ["from_tag"]
	}`)

	return core.Tool{
		Name:        "release_notes",
		Description: "Auto-generate release notes from git log between tags. Parses commits into feat/fix/chore/docs categories. Outputs markdown or JSON.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				FromTag string `json:"from_tag"`
				ToTag   string `json:"to_tag"`
				Format  string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.FromTag == "" {
				return core.ToolResult{}, fmt.Errorf("from_tag required")
			}
			if args.ToTag == "" {
				args.ToTag = "HEAD"
			}
			if args.Format == "" {
				args.Format = "markdown"
			}

			// Check git availability
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			// Get commits between tags
			rangeSpec := args.FromTag + ".." + args.ToTag
			cmd := exec.CommandContext(ctx, "git", "log", rangeSpec,
				"--format=%H||%an||%ad||%s", "--date=short")
			cmd.Dir = WorkspaceRoot
			out, err := cmd.Output()
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("git log failed: %w (check tags exist)", err)
			}

			commits := parseGitLog(string(out))
			categorized := categorizeCommits(commits)

			switch args.Format {
			case "json":
				return formatReleaseNotesJSON(args.FromTag, args.ToTag, categorized)
			default:
				return formatReleaseNotesMarkdown(args.FromTag, args.ToTag, categorized)
			}
		},
	}
}

// ChangelogTool creates a changelog generator tool.
// Parameters: from, to (semver), format (keepachangelog/markdown).
// Generates Keep a Changelog format. Categories: Added/Changed/Deprecated/Removed/Fixed/Security.
func ChangelogTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"from": {"type": "string", "description": "Starting version/tag (e.g., v1.0.0)"},
			"to": {"type": "string", "description": "Ending version/tag (e.g., v1.1.0, default: HEAD)"},
			"format": {"type": "string", "description": "Output format: keepachangelog, markdown (default: keepachangelog)"},
			"version": {"type": "string", "description": "Version for the release heading (default: to)"}
		},
		"required": ["from"]
	}`)

	return core.Tool{
		Name:        "changelog",
		Description: "Generate changelog in Keep a Changelog format from git log between versions. Categories: Added, Changed, Deprecated, Removed, Fixed, Security.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				From    string `json:"from"`
				To      string `json:"to"`
				Format  string `json:"format"`
				Version string `json:"version"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.From == "" {
				return core.ToolResult{}, fmt.Errorf("from version/tag required")
			}
			if args.To == "" {
				args.To = "HEAD"
			}
			if args.Format == "" {
				args.Format = "keepachangelog"
			}
			if args.Version == "" {
				args.Version = args.To
			}

			// Check git availability
			if _, err := exec.LookPath("git"); err != nil {
				return core.ToolResult{}, fmt.Errorf("git not available")
			}

			// Get commits between versions
			rangeSpec := args.From + ".." + args.To
			cmd := exec.CommandContext(ctx, "git", "log", rangeSpec,
				"--format=%H||%an||%ad||%s", "--date=short")
			cmd.Dir = WorkspaceRoot
			out, err := cmd.Output()
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("git log failed: %w (check tags exist)", err)
			}

			commits := parseGitLog(string(out))
			changelog := categorizeKeepAChangelog(commits)

			switch args.Format {
			case "markdown":
				return formatChangelogMarkdown(args.Version, changelog)
			default:
				return formatKeepAChangelog(args.Version, changelog)
			}
		},
	}
}

type commitEntry struct {
	Hash    string
	Author  string
	Date    string
	Message string
}

type releaseCategories struct {
	Features []commitEntry
	Fixes    []commitEntry
	Chores   []commitEntry
	Docs     []commitEntry
	Breaking []commitEntry
	Other    []commitEntry
}

type keepAChangelog struct {
	Added      []commitEntry
	Changed    []commitEntry
	Deprecated []commitEntry
	Removed    []commitEntry
	Fixed      []commitEntry
	Security   []commitEntry
}

func parseGitLog(output string) []commitEntry {
	var commits []commitEntry
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "||", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, commitEntry{
			Hash:    parts[0][:7],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}
	return commits
}

func categorizeCommits(commits []commitEntry) releaseCategories {
	var cats releaseCategories
	featRe := regexp.MustCompile(`^(feat|feature)[\(:!\]]`)
	fixRe := regexp.MustCompile(`^(fix|bugfix|hotfix)[\(:!\]]`)
	docsRe := regexp.MustCompile(`^(docs|doc)[\(:!\]]`)
	choreRe := regexp.MustCompile(`^(chore|refactor|style|test|ci|build|perf)[\(:!\]]`)
	breakingRe := regexp.MustCompile(`BREAKING CHANGE|!:`)

	for _, c := range commits {
		msg := strings.ToLower(c.Message)
		switch {
		case breakingRe.MatchString(c.Message):
			cats.Breaking = append(cats.Breaking, c)
		case featRe.MatchString(msg):
			cats.Features = append(cats.Features, c)
		case fixRe.MatchString(msg):
			cats.Fixes = append(cats.Fixes, c)
		case docsRe.MatchString(msg):
			cats.Docs = append(cats.Docs, c)
		case choreRe.MatchString(msg):
			cats.Chores = append(cats.Chores, c)
		default:
			// Heuristic: messages with "add", "new", "introduce" → features
			if strings.Contains(msg, "add") || strings.Contains(msg, "new") || strings.Contains(msg, "introduce") {
				cats.Features = append(cats.Features, c)
			} else if strings.Contains(msg, "fix") || strings.Contains(msg, "bug") || strings.Contains(msg, "resolv") {
				cats.Fixes = append(cats.Fixes, c)
			} else {
				cats.Other = append(cats.Other, c)
			}
		}
	}
	return cats
}

func categorizeKeepAChangelog(commits []commitEntry) keepAChangelog {
	var cl keepAChangelog
	for _, c := range commits {
		msg := strings.ToLower(c.Message)
		switch {
		case strings.Contains(c.Message, "BREAKING CHANGE") || strings.Contains(c.Message, "!:"):
			cl.Changed = append(cl.Changed, c)
		case strings.HasPrefix(msg, "feat") || strings.HasPrefix(msg, "feature"):
			cl.Added = append(cl.Added, c)
		case strings.HasPrefix(msg, "fix") || strings.HasPrefix(msg, "bugfix") || strings.HasPrefix(msg, "hotfix"):
			cl.Fixed = append(cl.Fixed, c)
		case strings.HasPrefix(msg, "security") || strings.Contains(msg, "vuln") || strings.Contains(msg, "CVE"):
			cl.Security = append(cl.Security, c)
		case strings.HasPrefix(msg, "deprecat"):
			cl.Deprecated = append(cl.Deprecated, c)
		case strings.HasPrefix(msg, "remov"):
			cl.Removed = append(cl.Removed, c)
		case strings.HasPrefix(msg, "refactor") || strings.HasPrefix(msg, "perf") || strings.HasPrefix(msg, "style"):
			cl.Changed = append(cl.Changed, c)
		default:
			// Heuristic fallback
			if strings.Contains(msg, "add") || strings.Contains(msg, "new") {
				cl.Added = append(cl.Added, c)
			} else if strings.Contains(msg, "fix") || strings.Contains(msg, "bug") {
				cl.Fixed = append(cl.Fixed, c)
			} else {
				cl.Changed = append(cl.Changed, c)
			}
		}
	}
	return cl
}

func formatReleaseNotesMarkdown(from, to string, cats releaseCategories) (core.ToolResult, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Release Notes: %s → %s\n\n", from, to))

	total := len(cats.Features) + len(cats.Fixes) + len(cats.Chores) + len(cats.Docs) + len(cats.Breaking) + len(cats.Other)
	sb.WriteString(fmt.Sprintf("_%d commits_\n\n", total))

	writeCategory := func(title string, entries []commitEntry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", title))
		for _, c := range entries {
			sb.WriteString(fmt.Sprintf("- **%s** %s (%s, %s)\n", c.Hash, cleanMessage(c.Message), c.Author, c.Date))
		}
		sb.WriteString("\n")
	}

	writeCategory("🚨 Breaking Changes", cats.Breaking)
	writeCategory("✨ Features", cats.Features)
	writeCategory("🐛 Bug Fixes", cats.Fixes)
	writeCategory("📚 Documentation", cats.Docs)
	writeCategory("🔧 Chores & Maintenance", cats.Chores)
	writeCategory("📦 Other", cats.Other)

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"from": from, "to": to, "total": total},
	}, nil
}

func formatReleaseNotesJSON(from, to string, cats releaseCategories) (core.ToolResult, error) {
	type commitJSON struct {
		Hash    string `json:"hash"`
		Author  string `json:"author"`
		Date    string `json:"date"`
		Message string `json:"message"`
	}

	toJSON := func(entries []commitEntry) []commitJSON {
		result := make([]commitJSON, len(entries))
		for i, c := range entries {
			result[i] = commitJSON{Hash: c.Hash, Author: c.Author, Date: c.Date, Message: c.Message}
		}
		return result
	}

	output := map[string]any{
		"from": from,
		"to":   to,
		"breaking": toJSON(cats.Breaking),
		"features": toJSON(cats.Features),
		"fixes":    toJSON(cats.Fixes),
		"docs":     toJSON(cats.Docs),
		"chores":   toJSON(cats.Chores),
		"other":    toJSON(cats.Other),
	}

	data, _ := json.MarshalIndent(output, "", "  ")
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: string(data)}},
		Details: map[string]any{"from": from, "to": to, "format": "json"},
	}, nil
}

func formatKeepAChangelog(version string, cl keepAChangelog) (core.ToolResult, error) {
	var sb strings.Builder
	sb.WriteString("# Changelog\n\n")
	sb.WriteString(fmt.Sprintf("## [%s]\n\n", version))

	total := len(cl.Added) + len(cl.Changed) + len(cl.Deprecated) + len(cl.Removed) + len(cl.Fixed) + len(cl.Security)

	writeSection := func(title string, entries []commitEntry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("### %s\n\n", title))
		for _, c := range entries {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", cleanMessage(c.Message), c.Hash))
		}
		sb.WriteString("\n")
	}

	writeSection("Added", cl.Added)
	writeSection("Changed", cl.Changed)
	writeSection("Deprecated", cl.Deprecated)
	writeSection("Removed", cl.Removed)
	writeSection("Fixed", cl.Fixed)
	writeSection("Security", cl.Security)

	if total == 0 {
		sb.WriteString("_No changes recorded._\n")
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"version": version, "total": total, "format": "keepachangelog"},
	}, nil
}

func formatChangelogMarkdown(version string, cl keepAChangelog) (core.ToolResult, error) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Changelog — %s\n\n", version))

	total := len(cl.Added) + len(cl.Changed) + len(cl.Deprecated) + len(cl.Removed) + len(cl.Fixed) + len(cl.Security)

	writeSection := func(title string, entries []commitEntry) {
		if len(entries) == 0 {
			return
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", title))
		for _, c := range entries {
			sb.WriteString(fmt.Sprintf("- %s\n", cleanMessage(c.Message)))
		}
		sb.WriteString("\n")
	}

	writeSection("✨ Added", cl.Added)
	writeSection("🔄 Changed", cl.Changed)
	writeSection("⚠️ Deprecated", cl.Deprecated)
	writeSection("🗑️ Removed", cl.Removed)
	writeSection("🐛 Fixed", cl.Fixed)
	writeSection("🔒 Security", cl.Security)

	if total == 0 {
		sb.WriteString("_No changes recorded._\n")
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: sb.String()}},
		Details: map[string]any{"version": version, "total": total, "format": "markdown"},
	}, nil
}

func cleanMessage(msg string) string {
	// Strip conventional commit prefixes for cleaner display
	re := regexp.MustCompile(`^(feat|feature|fix|bugfix|hotfix|docs|doc|chore|refactor|style|test|ci|build|perf|security|deprecat|remov)[\(:!\]]\s*`)
	return re.ReplaceAllString(msg, "")
}
