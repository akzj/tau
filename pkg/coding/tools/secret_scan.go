package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// ── Secret patterns ──

type secretPattern struct {
	name     string
	re       *regexp.Regexp
	severity string // high, medium, low
}

var secretPatterns = []secretPattern{
	// AWS keys
	{name: "AWS Access Key ID", re: regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), severity: "high"},
	{name: "AWS Secret Key", re: regexp.MustCompile(`\b[0-9a-zA-Z/+]{40}\b`), severity: "high"},
	// GitHub tokens
	{name: "GitHub Token", re: regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{36,255}\b`), severity: "high"},
	{name: "GitHub Classic Token", re: regexp.MustCompile(`\bghp_[A-Za-z0-9_]{36}\b`), severity: "high"},
	// Generic API keys
	{name: "Generic API Key", re: regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"][^'"]{16,}['"]`), severity: "high"},
	{name: "Stripe Key", re: regexp.MustCompile(`\b(sk|rk)_(live|test)_[0-9a-zA-Z]{24,}\b`), severity: "high"},
	// Passwords
	{name: "Password Assignment", re: regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*['"][^'"]{4,}['"]`), severity: "medium"},
	// Tokens
	{name: "Token Assignment", re: regexp.MustCompile(`(?i)(token|secret|auth)\s*[:=]\s*['"][^'"]{8,}['"]`), severity: "medium"},
	// Private keys
	{name: "Private Key Header", re: regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`), severity: "critical"},
	{name: "JWT Token", re: regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), severity: "high"},
	// GCP
	{name: "GCP Service Account", re: regexp.MustCompile(`"type":\s*"service_account"`), severity: "high"},
	// Generic connection strings
	{name: "Connection String", re: regexp.MustCompile(`(?i)(mongodb|postgres|mysql|redis)://[^'"\s]{10,}`), severity: "medium"},
	// Slack tokens
	{name: "Slack Token", re: regexp.MustCompile(`\bxox[baprs]-[0-9A-Za-z-]{10,}\b`), severity: "high"},
	// Heroku
	{name: "Heroku API Key", re: regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), severity: "low"},
	// Generic private key filename reference
	{name: "Private Key File Ref", re: regexp.MustCompile(`(?i)\.pem|id_rsa|id_ed25519|\.key\b`), severity: "low"},
}

// ── Skip directories ──

var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":      true,
	".svn":        true,
	".hg":         true,
	"testdata":    true,
	"dist":        true,
	"__pycache__": true,
	".tox":        true,
}

var skipExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".ico": true, ".svg": true,
	".mp3": true, ".mp4": true, ".avi": true, ".mov": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".7z": true,
	".pdf": true, ".doc": true, ".docx": true, ".ppt": true, ".xls": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
}

// SecretScanTool creates a secret/key scanning tool.
//
// Parameters:
//
//	path             (string, required)  — path to scan
//	exclude_patterns (string, optional)  — comma-separated glob patterns to exclude
//	format           (string, optional)  — json or text (default: text)
func SecretScanTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Directory or file path to scan"},
			"exclude_patterns": {"type": "string", "description": "Comma-separated glob patterns to exclude"},
			"format": {"type": "string", "description": "Output format: text or json (default: text)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "secret_scan",
		Description: "Secret/key scanner. Scans files for API keys, passwords, tokens, and private keys using regex patterns. Skips .git, node_modules, vendor. Returns file:line+secret_type+confidence.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path            string `json:"path"`
				ExcludePatterns string `json:"exclude_patterns"`
				Format          string `json:"format"`
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

			excludes := parseExcludePatterns(args.ExcludePatterns)

			findings := scanForSecrets(scanDir, excludes, onUpdate)

			if args.Format == "json" {
				return buildSecretScanJSON(findings), nil
			}
			return buildSecretScanText(findings), nil
		},
	}
}

type secretFinding struct {
	File       string `json:"file"`
	Line       int    `json:"line"`
	SecretType string `json:"secret_type"`
	Severity   string `json:"severity"`
	Match      string `json:"match"`
}

func parseExcludePatterns(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(raw, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func scanForSecrets(root string, excludes []string, onUpdate func(core.PartialResult)) []secretFinding {
	var findings []secretFinding

	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if skipDirs[name] {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && name != "." {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension skip
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if skipExts[ext] {
			return nil
		}

		// Check exclude patterns
		for _, ex := range excludes {
			if matched, _ := filepath.Match(ex, filepath.Base(path)); matched {
				return nil
			}
		}

		// Skip files larger than 1MB
		info, err := d.Info()
		if err != nil || info.Size() > 1<<20 {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		relPath, _ := filepath.Rel(root, path)

		for _, sp := range secretPatterns {
			matches := sp.re.FindAllStringSubmatchIndex(content, -1)
			for _, m := range matches {
				lineNum := lineNumberOf(content, m[0])
				matchText := content[m[0]:m[1]]
				// Truncate long matches for display
				if len(matchText) > 80 {
					matchText = matchText[:80] + "..."
				}
				findings = append(findings, secretFinding{
					File:       relPath,
					Line:       lineNum,
					SecretType: sp.name,
					Severity:   sp.severity,
					Match:      maskSecret(matchText),
				})
			}
		}
		return nil
	})

	return findings
}

func lineNumberOf(content string, byteOffset int) int {
	return strings.Count(content[:byteOffset], "\n") + 1
}

func maskSecret(s string) string {
	if len(s) <= 12 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}

func buildSecretScanText(findings []secretFinding) core.ToolResult {
	if len(findings) == 0 {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No secrets detected."}},
			Details: map[string]any{"finding_count": 0, "success": true, "findings": []secretFinding{}},
		}
	}

	var lines []string
	critical, high, medium, low := 0, 0, 0, 0
	for _, f := range findings {
		switch f.Severity {
		case "critical":
			critical++
		case "high":
			high++
		case "medium":
			medium++
		case "low":
			low++
		}
	}

	lines = append(lines, fmt.Sprintf("⚠ %d potential secret(s) found", len(findings)))
	lines = append(lines, fmt.Sprintf("  Critical: %d, High: %d, Medium: %d, Low: %d\n", critical, high, medium, low))

	for _, f := range findings {
		lines = append(lines, fmt.Sprintf("  [%s] %s: %s:%d — %s", strings.ToUpper(f.Severity), f.SecretType, f.File, f.Line, f.Match))
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"finding_count": len(findings),
			"critical":      critical,
			"high":          high,
			"medium":        medium,
			"low":           low,
			"success":       false,
			"findings":      findings,
		},
	}
}

func buildSecretScanJSON(findings []secretFinding) core.ToolResult {
	data, _ := json.MarshalIndent(findings, "", "  ")
	output := string(data)
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}
	details := map[string]any{
		"finding_count": len(findings),
		"success":       len(findings) == 0,
		"findings":      findings,
	}
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: details,
	}
}
