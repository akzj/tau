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

// AuditLicenseTool creates a license compliance checking tool.
//
// Parameters:
//
//	path             (string, required) — path to the Go module/project root
//	allowed_licenses (string, optional) — comma-separated list of allowed licenses
//	                                      (default: MIT,Apache-2.0,BSD-3-Clause,BSD-2-Clause,MPL-2.0)
func AuditLicenseTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the Go module/project root"},
			"allowed_licenses": {"type": "string", "description": "Comma-separated list of allowed licenses (e.g., MIT,Apache-2.0,BSD-3-Clause)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "audit_license",
		Description: "License compliance check. Scans go.mod dependencies, checks licenses using go-licenses or LICENSE file heuristics, and returns dependency+license+allowed status.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path            string `json:"path"`
				AllowedLicenses string `json:"allowed_licenses"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}

			allowed := parseAllowedLicenses(args.AllowedLicenses)

			// Try go-licenses first
			if _, err := exec.LookPath("go-licenses"); err == nil {
				return auditWithGoLicenses(ctx, args.Path, allowed)
			}

			// Fall back to heuristic scanning
			return auditHeuristic(ctx, args.Path, allowed)
		},
	}
}

type licenseEntry struct {
	Dependency string `json:"dependency"`
	License    string `json:"license"`
	Allowed    bool   `json:"allowed"`
	Source     string `json:"source"` // "go-licenses" or "heuristic"
}

func parseAllowedLicenses(s string) []string {
	if s == "" {
		return []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "MPL-2.0"}
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func isLicenseAllowed(license string, allowed []string) bool {
	license = strings.TrimSpace(license)
	for _, a := range allowed {
		if strings.EqualFold(a, license) {
			return true
		}
		// Check prefix for BSD variants
		if strings.HasPrefix(strings.ToLower(license), strings.ToLower(a)) {
			return true
		}
	}
	return false
}

func auditWithGoLicenses(ctx context.Context, path string, allowed []string) (core.ToolResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "go-licenses", "csv", path+"/...")
	cmd.Dir = path
	output, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(output))
	if err != nil && text == "" {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: fmt.Sprintf("go-licenses failed: %v", err)}},
			Details: map[string]any{"path": path, "success": false},
		}, err
	}
	if text == "" {
		return core.ToolResult{
			Content: []core.Content{{Type: "text", Text: "No dependencies found or go-licenses returned empty."}},
			Details: map[string]any{"path": path, "success": true, "entries": []licenseEntry{}},
		}, nil
	}

	return parseGoLicensesCSV(text, allowed)
}

func parseGoLicensesCSV(text string, allowed []string) (core.ToolResult, error) {
	var entries []licenseEntry
	violations := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "module,") {
			continue
		}
		parts := strings.SplitN(line, ",", 3)
		if len(parts) < 2 {
			continue
		}
		dep := strings.TrimSpace(parts[0])
		lic := strings.TrimSpace(parts[1])
		ok := isLicenseAllowed(lic, allowed)
		if !ok {
			violations++
		}
		entries = append(entries, licenseEntry{
			Dependency: dep,
			License:    lic,
			Allowed:    ok,
			Source:     "go-licenses",
		})
	}

	return buildLicenseResult(entries, violations), nil
}

func auditHeuristic(ctx context.Context, path string, allowed []string) (core.ToolResult, error) {
	// Scan vendor or GOPATH/pkg/mod for LICENSE files
	var entries []licenseEntry
	violations := 0

	goModPath := filepath.Join(path, "go.mod")
	data, err := os.ReadFile(goModPath)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("cannot read go.mod: %w", err)
	}

	deps := parseGoModDeps(string(data))
	for _, dep := range deps {
		lic := detectLicenseForDep(path, dep)
		ok := isLicenseAllowed(lic, allowed)
		if !ok {
			violations++
		}
		entries = append(entries, licenseEntry{
			Dependency: dep,
			License:    lic,
			Allowed:    ok,
			Source:     "heuristic",
		})
	}

	return buildLicenseResult(entries, violations), nil
}

func buildLicenseResult(entries []licenseEntry, violations int) core.ToolResult {
	var lines []string
	for _, e := range entries {
		status := "✓"
		if !e.Allowed {
			status = "✗"
		}
		lines = append(lines, fmt.Sprintf("%s %s\t%s\t[%s]", status, e.Dependency, e.License, e.Source))
	}

	output := strings.Join(lines, "\n")
	if output == "" {
		output = "No dependencies analyzed."
	}
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	success := violations == 0
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"dependency_count": len(entries),
			"violations":       violations,
			"success":          success,
			"entries":          entries,
		},
	}
}

func parseGoModDeps(content string) []string {
	var deps []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "require ") {
			// Single-line require
			rest := strings.TrimPrefix(line, "require ")
			parts := strings.Fields(rest)
			if len(parts) >= 1 {
				deps = append(deps, parts[0])
			}
		}
	}
	return deps
}

func detectLicenseForDep(projectRoot, dep string) string {
	// Check vendor directory first
	vendorPath := filepath.Join(projectRoot, "vendor", dep)
	if fi, err := os.Stat(vendorPath); err == nil && fi.IsDir() {
		return scanDirForLicense(vendorPath)
	}

	// Check GOPATH/pkg/mod
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	modPath := filepath.Join(gopath, "pkg", "mod", dep)
	if fi, err := os.Stat(modPath); err == nil && fi.IsDir() {
		return scanDirForLicense(modPath)
	}

	// Check GOMODCACHE
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		modPath := filepath.Join(gomodcache, dep)
		if fi, err := os.Stat(modPath); err == nil && fi.IsDir() {
			return scanDirForLicense(modPath)
		}
	}

	return "unknown"
}

func scanDirForLicense(dir string) string {
	candidates := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "LICENCE"}
	for _, c := range candidates {
		data, err := os.ReadFile(filepath.Join(dir, c))
		if err != nil {
			continue
		}
		return identifyLicense(string(data))
	}
	return "unknown"
}

func identifyLicense(text string) string {
	text = strings.ToLower(text[:min(len(text), 2000)])
	switch {
	case strings.Contains(text, "mit license") || strings.Contains(text, "permission is hereby granted"):
		return "MIT"
	case strings.Contains(text, "apache license") && strings.Contains(text, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(text, "bsd 3-clause") || strings.Contains(text, "redistribution and use in source and binary forms") && strings.Contains(text, "neither the name"):
		return "BSD-3-Clause"
	case strings.Contains(text, "bsd 2-clause") || (strings.Contains(text, "redistribution and use") && !strings.Contains(text, "neither the name")):
		return "BSD-2-Clause"
	case strings.Contains(text, "gnu general public license") && strings.Contains(text, "version 3"):
		return "GPL-3.0"
	case strings.Contains(text, "gnu general public license") && strings.Contains(text, "version 2"):
		return "GPL-2.0"
	case strings.Contains(text, "mozilla public license") || strings.Contains(text, "mpl"):
		return "MPL-2.0"
	case strings.Contains(text, "isc license"):
		return "ISC"
	case strings.Contains(text, "unlicense"):
		return "Unlicense"
	default:
		return "unknown"
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}