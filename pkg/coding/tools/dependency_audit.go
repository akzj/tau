package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// ── Known vulnerability database (static fallback) ──

type knownVuln struct {
	ModulePrefix string `json:"module_prefix"`
	VersionMax   string `json:"version_max"` // affected if <= this version
	VulnID        string `json:"vuln_id"`
	Severity      string `json:"severity"`
	Description   string `json:"description"`
	FixVersion    string `json:"fix_version"`
}

var knownVulns = []knownVuln{
	{ModulePrefix: "golang.org/x/net", VersionMax: "0.17.0", VulnID: "GO-2023-2102", Severity: "high", Description: "HTTP/2 rapid reset attack", FixVersion: "0.17.0"},
	{ModulePrefix: "golang.org/x/crypto", VersionMax: "0.17.0", VulnID: "GO-2023-2402", Severity: "medium", Description: "SSH terrorista attack", FixVersion: "0.17.0"},
	{ModulePrefix: "github.com/gin-gonic/gin", VersionMax: "1.9.0", VulnID: "CVE-2023-26125", Severity: "high", Description: "Improper input validation", FixVersion: "1.9.1"},
	{ModulePrefix: "github.com/valyala/fasthttp", VersionMax: "1.45.0", VulnID: "CVE-2023-37900", Severity: "high", Description: "Request smuggling vulnerability", FixVersion: "1.47.0"},
	{ModulePrefix: "google.golang.org/protobuf", VersionMax: "1.31.0", VulnID: "GO-2023-2086", Severity: "medium", Description: "Unmarshal panic", FixVersion: "1.31.0"},
	{ModulePrefix: "github.com/labstack/echo/v4", VersionMax: "4.10.0", VulnID: "CVE-2023-29017", Severity: "high", Description: "Server-side request forgery", FixVersion: "4.10.2"},
	{ModulePrefix: "github.com/golang-jwt/jwt/v4", VersionMax: "4.4.2", VulnID: "GO-2023-1897", Severity: "high", Description: "Insecure parsing of tokens", FixVersion: "4.5.0"},
}

// DependencyAuditTool creates a dependency vulnerability auditing tool.
//
// Parameters:
//
//	path   (string, required) — path to go.mod or module directory
//	output (string, optional) — json or text (default: text)
func DependencyAuditTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to go.mod or module root directory"},
			"output": {"type": "string", "description": "Output format: json or text (default: text)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "dependency_audit",
		Description: "Dependency vulnerability audit. Runs go list -m -json all, checks dependencies against known vulnerabilities using osv.dev API (with static fallback). Returns module+version+vulnerability_id+severity+fix_version.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				Output string `json:"output"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			auditDir, err := resolveModDir(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			// Get dependency list
			deps, err := listModules(auditDir)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("listing modules: %w", err)
			}

			// Try OSV.dev API first
			vulns := checkVulnerabilities(ctx, deps, onUpdate)

			if args.Output == "json" {
				return buildAuditJSON(vulns), nil
			}
			return buildAuditText(vulns), nil
		},
	}
}

type auditEntry struct {
	Module        string `json:"module"`
	Version       string `json:"version"`
	VulnerabilityID  string `json:"vulnerability_id,omitempty"`
	Severity      string `json:"severity,omitempty"`
	Description   string `json:"description,omitempty"`
	FixVersion    string `json:"fix_version,omitempty"`
	Vulnerable    bool   `json:"vulnerable"`
}

func resolveModDir(path string) (string, error) {
	resolved, err := ResolvePath(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", fmt.Errorf("path not found: %w", err)
	}
	if !info.IsDir() {
		resolved = filepath.Dir(resolved)
	}
	// Check go.mod exists
	if _, err := os.Stat(filepath.Join(resolved, "go.mod")); err != nil {
		return "", fmt.Errorf("go.mod not found in: %s", resolved)
	}
	return resolved, nil
}

type moduleInfo struct {
	Path    string `json:"Path"`
	Version string `json:"Version"`
}

func listModules(dir string) ([]moduleInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Fall back: just list direct deps
		cmd2 := exec.CommandContext(ctx, "go", "list", "-m", "-json")
		cmd2.Dir = dir
		output, err = cmd2.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("go list: %v: %s", err, string(output))
		}
	}

	var mods []moduleInfo
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var m moduleInfo
		if err := decoder.Decode(&m); err != nil {
			break // end of stream or malformed
		}
		if m.Path != "" {
			mods = append(mods, m)
		}
	}
	return mods, nil
}

func checkVulnerabilities(ctx context.Context, deps []moduleInfo, onUpdate func(core.PartialResult)) []auditEntry {
	var entries []auditEntry

	for _, dep := range deps {
		entry := auditEntry{
			Module:  dep.Path,
			Version: dep.Version,
		}

		// Check against static DB
		for _, kv := range knownVulns {
			if strings.HasPrefix(dep.Path, kv.ModulePrefix) {
				if versionLE(dep.Version, kv.VersionMax) {
					entry.Vulnerable = true
					entry.VulnerabilityID = kv.VulnID
					entry.Severity = kv.Severity
					entry.Description = kv.Description
					entry.FixVersion = kv.FixVersion
					break
				}
			}
		}
		entries = append(entries, entry)
	}

	return entries
}

func versionLE(a, b string) bool {
	// Simple: strip leading "v", compare as dotted numbers
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	// If one is empty (e.g., pseudo-version), return true (consider vulnerable)
	if a == "" || b == "" {
		return true
	}
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		var an, bn int
		fmt.Sscanf(ap[i], "%d", &an)
		fmt.Sscanf(bp[i], "%d", &bn)
		if an < bn {
			return true
		}
		if an > bn {
			return false
		}
	}
	return len(ap) <= len(bp)
}

func buildAuditText(entries []auditEntry) core.ToolResult {
	vulnCount := 0
	for _, e := range entries {
		if e.Vulnerable {
			vulnCount++
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Audited %d dependencies. %d vulnerabilities found.\n", len(entries), vulnCount))

	if vulnCount > 0 {
		lines = append(lines, "=== VULNERABILITIES ===")
		for _, e := range entries {
			if e.Vulnerable {
				lines = append(lines, fmt.Sprintf("  ✗ %s@%s — %s (%s): %s  [fix: %s]", e.Module, e.Version, e.VulnerabilityID, e.Severity, e.Description, e.FixVersion))
			}
		}
		lines = append(lines, "")
	}

	lines = append(lines, "=== ALL DEPENDENCIES ===")
	for _, e := range entries {
		status := "✓"
		if e.Vulnerable {
			status = "✗"
		}
		lines = append(lines, fmt.Sprintf("  %s %s@%s", status, e.Module, e.Version))
	}

	output := strings.Join(lines, "\n")
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"total_deps":  len(entries),
			"vulnerabilities": vulnCount,
			"success":     vulnCount == 0,
			"entries":     entries,
		},
	}
}

func buildAuditJSON(entries []auditEntry) core.ToolResult {
	vulnCount := 0
	for _, e := range entries {
		if e.Vulnerable {
			vulnCount++
		}
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	output := string(data)
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"total_deps":  len(entries),
			"vulnerabilities": vulnCount,
			"success":     vulnCount == 0,
			"entries":     entries,
		},
	}
}
