package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// ── Built-in regex patterns (fallback when gitleaks not installed) ──

type leakPattern struct {
	ID          string
	Description string
	Regex       *regexp.Regexp
}

var builtinLeakPatterns = []leakPattern{
	{ID: "aws-access-key", Description: "AWS Access Key", Regex: regexp.MustCompile(`\b(AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{ID: "aws-secret-key", Description: "AWS Secret Key", Regex: regexp.MustCompile(`\b[0-9a-zA-Z/+]{40}\b`)},
	{ID: "github-token", Description: "GitHub Personal Access Token", Regex: regexp.MustCompile(`\bghp_[A-Za-z0-9_]{36}\b`)},
	{ID: "github-oauth", Description: "GitHub OAuth Token", Regex: regexp.MustCompile(`\bgho_[A-Za-z0-9_]{36,}\b`)},
	{ID: "gitlab-token", Description: "GitLab Access Token", Regex: regexp.MustCompile(`\bglpat-[A-Za-z0-9_-]{20,}\b`)},
	{ID: "private-key", Description: "Private Key", Regex: regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)},
	{ID: "jwt-token", Description: "JWT Token", Regex: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
	{ID: "google-api-key", Description: "Google API Key", Regex: regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
	{ID: "stripe-key", Description: "Stripe API Key", Regex: regexp.MustCompile(`\b(sk|rk)_(live|test)_[0-9a-zA-Z]{24,}\b`)},
	{ID: "slack-token", Description: "Slack Bot Token", Regex: regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`)},
	{ID: "generic-password", Description: "Generic Password/Secret Assignment", Regex: regexp.MustCompile(`(?i)(password|passwd|pwd|secret)\s*[:=]\s*['"][^'"]{4,}['"]`)},
	{ID: "connection-string", Description: "Database Connection String", Regex: regexp.MustCompile(`(?i)(mongodb|postgres|mysql|redis)://[^'"\s]{8,}`)},
}

// GitleaksScanTool creates a gitleaks-based secret scanning tool.
//
// Parameters:
//
//	path     (string, required) — repository path to scan
//	verbose  (boolean, optional) — enable verbose output
//	no_git   (boolean, optional) — scan without git history (files only)
func GitleaksScanTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Repository path to scan"},
			"verbose": {"type": "boolean", "description": "Enable verbose output (default: false)"},
			"no_git": {"type": "boolean", "description": "Scan files without git history (default: false)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "gitleaks_scan",
		Description: "GitLeaks secret scanner. Runs gitleaks detect on repository. Falls back to built-in regex scanner if gitleaks not installed. Returns leak count and findings list.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path    string `json:"path"`
				Verbose bool   `json:"verbose"`
				NoGit   bool   `json:"no_git"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			scanDir, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			// Try gitleaks first
			if _, lookErr := exec.LookPath("gitleaks"); lookErr == nil {
				return runGitleaks(ctx, scanDir, args.Verbose, args.NoGit)
			}

			// Fall back to built-in regex scanner
			return runBuiltinLeakScan(scanDir, args.Verbose, onUpdate)
		},
	}
}

type gitleaksFinding struct {
	Description string `json:"description"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	RuleID      string `json:"rule_id"`
	Match       string `json:"match"`
	Severity    string `json:"severity"`
}

func runGitleaks(ctx context.Context, dir string, verbose, noGit bool) (core.ToolResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	args := []string{"detect", "--source", dir, "--no-banner"}
	if verbose {
		args = append(args, "-v")
	}
	if noGit {
		args = append(args, "--no-git")
	}
	// Always output JSON for parsing
	args = append(args, "-f", "json", "-r", "-")

	cmd := exec.CommandContext(timeoutCtx, "gitleaks", args...)
	output, err := cmd.CombinedOutput()

	// gitleaks returns exit code 1 when leaks are found
	hasLeaks := err != nil

	// Parse JSON output
	findings := parseGitleaksJSON(string(output))

	if len(findings) == 0 && hasLeaks {
		// Couldn't parse, return raw output
		text := string(output)
		if len(text) > OutputCap {
			text = text[:OutputCap] + "\n... (truncated)"
		}
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: text}},
			Details: map[string]any{"leak_count": 0, "success": false, "engine": "gitleaks", "raw": true},
		}, nil
	}

	return buildGitleaksResult(findings, "gitleaks"), nil
}

func parseGitleaksJSON(raw string) []gitleaksFinding {
	var findings []gitleaksFinding
	raw = strings.TrimSpace(raw)

	// Unwrap JSON array: use Token to read opening [, then decode elements, then ]
	decoder := json.NewDecoder(strings.NewReader(raw))
	tok, err := decoder.Token()
	if err != nil {
		return findings
	}
	// If the first token is [, it's an array — decode elements individually
	if delim, ok := tok.(json.Delim); ok && delim == '[' {
		for decoder.More() {
			var f gitleaksFinding
			if err := decoder.Decode(&f); err != nil {
				break
			}
			if f.RuleID != "" {
				findings = append(findings, f)
			}
		}
		return findings
	}
	// Single object fallback
	var f gitleaksFinding
	if err := json.Unmarshal([]byte(raw), &f); err == nil && f.RuleID != "" {
		findings = append(findings, f)
	}
	return findings
}

func runBuiltinLeakScan(dir string, verbose bool, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
	var findings []gitleaksFinding

	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true,
		".svn": true, ".hg": true, "dist": true, ".tox": true,
	}

	skipExts := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true,
		".pdf": true, ".doc": true, ".docx": true, ".ppt": true, ".xls": true,
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	}

	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if skipExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil || info.Size() > 2<<20 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		relPath, _ := filepath.Rel(dir, path)

		for _, lp := range builtinLeakPatterns {
			matchLocs := lp.Regex.FindAllStringIndex(content, -1)
			for _, m := range matchLocs {
				matchText := content[m[0]:m[1]]
				if len(matchText) > 60 {
					matchText = matchText[:60] + "..."
				}
				lineNum := strings.Count(content[:m[0]], "\n") + 1
				findings = append(findings, gitleaksFinding{
					RuleID:      lp.ID,
					Description: lp.Description,
					File:        relPath,
					Line:        lineNum,
					Match:       matchText,
					Severity:    "high",
				})
			}
		}
		return nil
	})

	return buildGitleaksResult(findings, "builtin"), nil
}

func buildGitleaksResult(findings []gitleaksFinding, engine string) core.ToolResult {
	if len(findings) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No leaks detected."}},
			Details: map[string]any{"leak_count": 0, "success": true, "engine": engine, "findings": []gitleaksFinding{}},
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("⚠ %d potential leak(s) detected (engine: %s)\n", len(findings), engine))

	for _, f := range findings {
		lines = append(lines, fmt.Sprintf("  [%s] %s: %s:%d — %s", f.RuleID, f.Description, f.File, f.Line, f.Match))
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"leak_count": len(findings),
			"success":    false,
			"engine":     engine,
			"findings":   findings,
		},
	}
}
