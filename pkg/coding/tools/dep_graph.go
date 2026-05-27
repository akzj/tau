package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/akzj/tau/core"
)

// DepGraphTool creates a tool that generates dependency graphs for Go packages.
func DepGraphTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Package path (default: ./...)"},
			"output_format": {"type": "string", "description": "Output format: mermaid, dot, json, or text (default: text)"},
			"include_external": {"type": "boolean", "description": "Include external module deps (default: false)"},
			"max_depth": {"type": "integer", "description": "Max depth for dependency traversal (default: 5)"}
		}
	}`)

	return core.Tool{
		Name:        "dep_graph",
		Description: "Generate a dependency graph for Go packages using go list.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path            string `json:"path"`
				OutputFormat    string `json:"output_format"`
				IncludeExternal bool   `json:"include_external"`
				MaxDepth        int    `json:"max_depth"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.Path == "" {
				args.Path = "./..."
			}
			if args.OutputFormat == "" {
				args.OutputFormat = "text"
			}
			if args.MaxDepth <= 0 {
				args.MaxDepth = 5
			}

			pkgs, err := listPackages(ctx, args.Path)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("go list failed: %w", err)
			}
			if len(pkgs) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "No packages found."}},
				}, nil
			}

			// Determine the module path from the first package.
			modulePath := findModulePath(pkgs)
			isExternal := makeExternalFilter(pkgs, modulePath, args.IncludeExternal)

			var output string
			switch args.OutputFormat {
			case "json":
				output = formatDepJSON(pkgs, isExternal)
			case "mermaid":
				output = formatMermaid(pkgs, isExternal, args.MaxDepth)
			case "dot":
				output = formatDot(pkgs, isExternal, args.MaxDepth)
			default:
				output = formatText(pkgs, isExternal, args.MaxDepth)
			}

			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"path":     args.Path,
					"packages": len(pkgs),
					"format":   args.OutputFormat,
				},
			}, nil
		},
	}
}

// pkgInfo holds the parsed fields from one "go list -json" object.
type pkgInfo struct {
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	Deps       []string `json:"Deps"`
	Standard   bool     `json:"Standard"`
}

// listPackages runs "go list -json -deps <path>" and returns all parsed packages.
func listPackages(ctx context.Context, path string) ([]pkgInfo, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "-deps", path)
	cmd.Dir = WorkspaceRoot
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// go list may write to stderr on failure; include it.
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
		}
		return nil, err
	}

	dec := json.NewDecoder(&stdout)
	var pkgs []pkgInfo
	for dec.More() {
		var p pkgInfo
		if err := dec.Decode(&p); err != nil {
			break
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

// findModulePath returns the module path (longest common prefix of non-stdlib import paths).
func findModulePath(pkgs []pkgInfo) string {
	var nonStd []string
	for _, p := range pkgs {
		if !p.Standard {
			nonStd = append(nonStd, p.ImportPath)
		}
	}
	if len(nonStd) == 0 {
		return ""
	}
	prefix := nonStd[0]
	for _, p := range nonStd[1:] {
		for !strings.HasPrefix(p, prefix) {
			idx := strings.LastIndex(prefix, "/")
			if idx < 0 {
				return ""
			}
			prefix = prefix[:idx]
		}
	}
	return prefix
}

// makeExternalFilter returns a predicate: true when a package should be displayed.
func makeExternalFilter(pkgs []pkgInfo, modulePath string, includeExternal bool) func(string) bool {
	lookup := make(map[string]pkgInfo, len(pkgs))
	for _, p := range pkgs {
		lookup[p.ImportPath] = p
	}
	return func(importPath string) bool {
		p, ok := lookup[importPath]
		if !ok {
			return includeExternal
		}
		if p.Standard {
			return true // always show stdlib
		}
		if modulePath != "" && strings.HasPrefix(importPath, modulePath) {
			return true // internal package
		}
		return includeExternal
	}
}

// subsetDeps returns the dep list filtered through the predicate.
func subsetDeps(all []string, filter func(string) bool) []string {
	var out []string
	for _, d := range all {
		if filter(d) {
			out = append(out, d)
		}
	}
	sort.Strings(out)
	return out
}

// ---------- text ----------

func formatText(pkgs []pkgInfo, filter func(string) bool, maxDepth int) string {
	rootPaths := rootImportPaths(pkgs)
	index := make(map[string]pkgInfo, len(pkgs))
	for _, p := range pkgs {
		index[p.ImportPath] = p
	}

	var b strings.Builder
	seen := make(map[string]bool)
	for _, root := range rootPaths {
		writeTextTree(&b, index, filter, root, "", seen, 0, maxDepth)
	}
	return b.String()
}

func writeTextTree(b *strings.Builder, index map[string]pkgInfo, filter func(string) bool,
	importPath, prefix string, seen map[string]bool, depth, maxDepth int) {
	if depth > maxDepth {
		return
	}
	if seen[importPath] {
		b.WriteString(fmt.Sprintf("%s%s (already shown)\n", prefix, shortName(importPath)))
		return
	}
	seen[importPath] = true

	p, ok := index[importPath]
	if !ok {
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, importPath))
		return
	}

	label := shortName(importPath)
	if p.Standard {
		label += " (stdlib)"
	} else if !strings.HasPrefix(importPath, findModulePathFromIndex(index)) {
		label += " (external)"
	}
	b.WriteString(fmt.Sprintf("%s%s\n", prefix, label))

	deps := subsetDeps(p.Deps, filter)
	for i, dep := range deps {
		connector := "├── "
		childPrefix := prefix + "│   "
		if i == len(deps)-1 {
			connector = "└── "
			childPrefix = prefix + "    "
		}
		b.WriteString(prefix + connector)
		writeTextTree(b, index, filter, dep, childPrefix, seen, depth+1, maxDepth)
	}
}

// ---------- mermaid ----------

func formatMermaid(pkgs []pkgInfo, filter func(string) bool, maxDepth int) string {
	rootPaths := rootImportPaths(pkgs)
	index := make(map[string]pkgInfo, len(pkgs))
	for _, p := range pkgs {
		index[p.ImportPath] = p
	}

	var b strings.Builder
	b.WriteString("```mermaid\ngraph TD\n")
	visited := make(map[string]bool)
	for _, root := range rootPaths {
		writeMermaidEdges(&b, index, filter, root, visited, 0, maxDepth)
	}
	b.WriteString("```")
	return b.String()
}

func writeMermaidEdges(b *strings.Builder, index map[string]pkgInfo, filter func(string) bool,
	importPath string, visited map[string]bool, depth, maxDepth int) {
	if depth > maxDepth || visited[importPath] {
		return
	}
	visited[importPath] = true

	p, ok := index[importPath]
	if !ok {
		return
	}
	fromID := mermaidID(importPath)
	for _, dep := range subsetDeps(p.Deps, filter) {
		toID := mermaidID(dep)
		b.WriteString(fmt.Sprintf("    %s[\"%s\"] --> %s[\"%s\"]\n", fromID, shortName(importPath), toID, shortName(dep)))
		writeMermaidEdges(b, index, filter, dep, visited, depth+1, maxDepth)
	}
}

// ---------- dot ----------

func formatDot(pkgs []pkgInfo, filter func(string) bool, maxDepth int) string {
	rootPaths := rootImportPaths(pkgs)
	index := make(map[string]pkgInfo, len(pkgs))
	for _, p := range pkgs {
		index[p.ImportPath] = p
	}

	var b strings.Builder
	b.WriteString("digraph {\n")
	b.WriteString("    rankdir=LR;\n")
	visited := make(map[string]bool)
	for _, root := range rootPaths {
		writeDotEdges(&b, index, filter, root, visited, 0, maxDepth)
	}
	b.WriteString("}")
	return b.String()
}

func writeDotEdges(b *strings.Builder, index map[string]pkgInfo, filter func(string) bool,
	importPath string, visited map[string]bool, depth, maxDepth int) {
	if depth > maxDepth || visited[importPath] {
		return
	}
	visited[importPath] = true

	p, ok := index[importPath]
	if !ok {
		return
	}
	for _, dep := range subsetDeps(p.Deps, filter) {
		b.WriteString(fmt.Sprintf("    \"%s\" -> \"%s\";\n", importPath, dep))
		writeDotEdges(b, index, filter, dep, visited, depth+1, maxDepth)
	}
}

// ---------- json ----------

func formatDepJSON(pkgs []pkgInfo, filter func(string) bool) string {
	type depEntry struct {
		Package      string   `json:"package"`
		Dependencies []string `json:"dependencies"`
	}
	var entries []depEntry
	for _, p := range pkgs {
		deps := subsetDeps(p.Deps, filter)
		entries = append(entries, depEntry{
			Package:      p.ImportPath,
			Dependencies: deps,
		})
	}
	out, _ := json.MarshalIndent(entries, "", "  ")
	return string(out)
}

// ---------- helpers ----------

// rootImportPaths returns the import paths of root packages (those not listed as
// a dependency of any other package in the set). This naturally identifies the
// packages directly matched by "go list".
func rootImportPaths(pkgs []pkgInfo) []string {
	asDep := make(map[string]bool)
	for _, p := range pkgs {
		for _, d := range p.Deps {
			asDep[d] = true
		}
	}
	var roots []string
	for _, p := range pkgs {
		if !asDep[p.ImportPath] {
			roots = append(roots, p.ImportPath)
		}
	}
	sort.Strings(roots)
	if len(roots) == 0 && len(pkgs) > 0 {
		// Fallback: first package is the root.
		roots = []string{pkgs[0].ImportPath}
	}
	return roots
}

// shortName returns the last segment of an import path.
func shortName(importPath string) string {
	if idx := strings.LastIndex(importPath, "/"); idx >= 0 {
		return importPath[idx+1:]
	}
	return importPath
}

// mermaidID returns a safe Mermaid node identifier.
func mermaidID(importPath string) string {
	return "N" + strings.NewReplacer(
		".", "_", "/", "_", "-", "_",
	).Replace(importPath)
}

// findModulePathFromIndex is a fast variant that works from an index map.
func findModulePathFromIndex(index map[string]pkgInfo) string {
	var nonStd []string
	for _, p := range index {
		if !p.Standard {
			nonStd = append(nonStd, p.ImportPath)
		}
	}
	if len(nonStd) == 0 {
		return ""
	}
	prefix := nonStd[0]
	for _, p := range nonStd[1:] {
		for !strings.HasPrefix(p, prefix) {
			idx := strings.LastIndex(prefix, "/")
			if idx < 0 {
				return ""
			}
			prefix = prefix[:idx]
		}
	}
	return prefix
}

var _ = sort.Strings
var _ = filepath.Rel
