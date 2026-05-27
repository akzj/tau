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

// SBOMGenerateTool creates an SBOM (Software Bill of Materials) generator.
//
// Parameters:
//
//	path   (string, required) — path to go.mod or module directory
//	format (string, optional) — cyclonedx, spdx, or json (default: json)
//	output (string, optional) — output file path (default: stdout)
func SBOMGenerateTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to go.mod or module root directory"},
			"format": {"type": "string", "description": "Output format: cyclonedx, spdx, or json (default: json)"},
			"output": {"type": "string", "description": "Output file path (default: stdout)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "sbom_generate",
		Description: "SBOM generator. Generates Software Bill of Materials from go.mod dependencies in CycloneDX, SPDX, or JSON format. Includes component name, version, purl, license.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path   string `json:"path"`
				Format string `json:"format"`
				Output string `json:"output"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Format == "" {
				args.Format = "json"
			}

			sbomDir, err := resolveModDirForSBOM(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			components, err := collectComponents(sbomDir)
			if err != nil {
				return core.ToolResult{}, err
			}

			var outputText string
			switch args.Format {
			case "cyclonedx":
				outputText = generateCycloneDX(components)
			case "spdx":
				outputText = generateSPDX(components)
			default:
				outputText = generateJSONSBOM(components)
			}

			// Write to file if requested
			if args.Output != "" {
				outputPath, err := ResolvePath(args.Output)
				if err != nil {
					return core.ToolResult{}, err
				}
				if err := os.WriteFile(outputPath, []byte(outputText), 0644); err != nil {
					return core.ToolResult{}, fmt.Errorf("write output: %w", err)
				}
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("SBOM written to %s (%d components)", args.Output, len(components))}},
					Details: map[string]any{
						"components": len(components),
						"format":     args.Format,
						"output":     args.Output,
						"success":    true,
					},
				}, nil
			}

			if len(outputText) > OutputCap {
				outputText = outputText[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: outputText}},
				Details: map[string]any{
					"components": len(components),
					"format":     args.Format,
					"success":    true,
					"sbom":       components,
				},
			}, nil
		},
	}
}

type sbomComponent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl"`
	License string `json:"license,omitempty"`
}

func resolveModDirForSBOM(path string) (string, error) {
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
	if _, err := os.Stat(filepath.Join(resolved, "go.mod")); err != nil {
		return "", fmt.Errorf("go.mod not found in: %s", resolved)
	}
	return resolved, nil
}

func collectComponents(dir string) ([]sbomComponent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-m", "-json", "all")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go list: %v: %s", err, string(output))
	}

	var components []sbomComponent
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for decoder.More() {
		var m struct {
			Path    string `json:"Path"`
			Version string `json:"Version"`
		}
		if err := decoder.Decode(&m); err != nil {
			break
		}
		if m.Path == "" {
			continue
		}
		// Skip stdlib
		if !strings.Contains(m.Path, ".") {
			continue
		}

		lic := detectLicenseForSBOM(dir, m.Path)
		components = append(components, sbomComponent{
			Name:    m.Path,
			Version: strings.TrimPrefix(m.Version, "v"),
			PURL:    "pkg:golang/" + m.Path + "@" + strings.TrimPrefix(m.Version, "v"),
			License: lic,
		})
	}
	return components, nil
}

func detectLicenseForSBOM(projectRoot, dep string) string {
	// Check vendor directory first
	vendorPath := filepath.Join(projectRoot, "vendor", dep)
	if fi, err := os.Stat(vendorPath); err == nil && fi.IsDir() {
		return scanDirForLicenseSBOM(vendorPath)
	}

	// Check GOPATH / GOMODCACHE
	cachePaths := []string{}
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		cachePaths = append(cachePaths, filepath.Join(gomodcache, dep))
	}
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		home, _ := os.UserHomeDir()
		gopath = filepath.Join(home, "go")
	}
	cachePaths = append(cachePaths, filepath.Join(gopath, "pkg", "mod", dep))

	for _, cp := range cachePaths {
		if fi, err := os.Stat(cp); err == nil && fi.IsDir() {
			return scanDirForLicenseSBOM(cp)
		}
	}
	return ""
}

func scanDirForLicenseSBOM(dir string) string {
	candidates := []string{"LICENSE", "LICENSE.md", "LICENSE.txt", "COPYING", "LICENCE"}
	for _, c := range candidates {
		data, err := os.ReadFile(filepath.Join(dir, c))
		if err != nil {
			continue
		}
		return identifyLicenseFromContent(string(data))
	}
	return ""
}

func identifyLicenseFromContent(text string) string {
	lower := strings.ToLower(text[:minStrLen(len(text), 2000)])
	switch {
	case strings.Contains(lower, "mit license") || strings.Contains(lower, "permission is hereby granted"):
		return "MIT"
	case strings.Contains(lower, "apache license") && strings.Contains(lower, "version 2.0"):
		return "Apache-2.0"
	case strings.Contains(lower, "bsd 3-clause"):
		return "BSD-3-Clause"
	case strings.Contains(lower, "bsd 2-clause"):
		return "BSD-2-Clause"
	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 3"):
		return "GPL-3.0"
	case strings.Contains(lower, "gnu general public license") && strings.Contains(lower, "version 2"):
		return "GPL-2.0"
	case strings.Contains(lower, "mozilla public license") || strings.Contains(lower, "mpl"):
		return "MPL-2.0"
	case strings.Contains(lower, "isc license"):
		return "ISC"
	case strings.Contains(lower, "unlicense"):
		return "Unlicense"
	default:
		return ""
	}
}

func minStrLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ── Format generators ──

func generateJSONSBOM(components []sbomComponent) string {
	sbom := map[string]any{
		"bomFormat":    "tau-sbom",
		"specVersion":  "1.0",
		"components":   components,
	}
	data, _ := json.MarshalIndent(sbom, "", "  ")
	return string(data)
}

func generateCycloneDX(components []sbomComponent) string {
	var sb strings.Builder
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<bom xmlns=\"http://cyclonedx.org/schema/bom/1.4\" serialNumber=\"urn:uuid:" + generatePseudoUUID() + "\" version=\"1\">\n")
	sb.WriteString("  <metadata>\n")
	sb.WriteString("    <timestamp>" + time.Now().UTC().Format(time.RFC3339) + "</timestamp>\n")
	sb.WriteString("  </metadata>\n")
	sb.WriteString("  <components>\n")
	for _, c := range components {
		sb.WriteString(fmt.Sprintf("    <component type=\"library\">\n"))
		sb.WriteString(fmt.Sprintf("      <name>%s</name>\n", c.Name))
		sb.WriteString(fmt.Sprintf("      <version>%s</version>\n", c.Version))
		sb.WriteString(fmt.Sprintf("      <purl>%s</purl>\n", c.PURL))
		if c.License != "" {
			sb.WriteString(fmt.Sprintf("      <licenses><license><id>%s</id></license></licenses>\n", c.License))
		}
		sb.WriteString(fmt.Sprintf("    </component>\n"))
	}
	sb.WriteString("  </components>\n")
	sb.WriteString("</bom>\n")
	return sb.String()
}

func generateSPDX(components []sbomComponent) string {
	var sb2 strings.Builder
	sb2.WriteString("SPDXVersion: SPDX-2.3\n")
	sb2.WriteString("DataLicense: CC0-1.0\n")
	sb2.WriteString("SPDXID: SPDXRef-DOCUMENT\n")
	sb2.WriteString("DocumentName: go-module-sbom\n")
	sb2.WriteString("Creator: tau sbom_generate tool\n")
	sb2.WriteString(fmt.Sprintf("Created: %s\n\n", time.Now().UTC().Format(time.RFC3339)))
	for _, c := range components {
		sb2.WriteString(fmt.Sprintf("PackageName: %s\n", c.Name))
		sb2.WriteString(fmt.Sprintf("PackageVersion: %s\n", c.Version))
		sb2.WriteString(fmt.Sprintf("PackageDownloadLocation: NONE\n"))
		if c.License != "" {
			sb2.WriteString(fmt.Sprintf("PackageLicenseConcluded: %s\n", c.License))
		}
		sb2.WriteString(fmt.Sprintf("ExternalRef: PACKAGE-MANAGER purl %s\n\n", c.PURL))
	}
	return sb2.String()
}

func generatePseudoUUID() string {
	// Deterministic pseudo-UUID for SBOM serial numbers
	return "00000000-0000-4000-8000-000000000001"
}
