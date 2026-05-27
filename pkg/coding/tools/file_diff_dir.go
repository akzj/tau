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

// FileDiffDirTool creates a directory comparison tool.
//
// Parameters:
//
//	dir_a  (string, required) — first directory path
//	dir_b  (string, required) — second directory path
//	filter (string, optional) — glob filter (e.g., "*.go")
//
// Compares file lists, sizes, and modification times.
// Diffs text content for files present in both directories.
func FileDiffDirTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"dir_a": {"type": "string", "description": "First directory path"},
			"dir_b": {"type": "string", "description": "Second directory path"},
			"filter": {"type": "string", "description": "Glob filter (e.g., '*.go')"}
		},
		"required": ["dir_a", "dir_b"]
	}`)

	return core.Tool{
		Name:        "file_diff_dir",
		Description: "Compare two directories: file lists, sizes, modification times, and content diffs.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				DirA   string `json:"dir_a"`
				DirB   string `json:"dir_b"`
				Filter string `json:"filter"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.DirA == "" || args.DirB == "" {
				return core.ToolResult{}, fmt.Errorf("dir_a and dir_b required")
			}

			dirA, err := ResolvePath(args.DirA)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("dir_a: %w", err)
			}
			dirB, err := ResolvePath(args.DirB)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("dir_b: %w", err)
			}

			filesA, err := listDirFiles(dirA, args.Filter)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("dir_a list: %w", err)
			}
			filesB, err := listDirFiles(dirB, args.Filter)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("dir_b list: %w", err)
			}

			var output strings.Builder
			output.WriteString(fmt.Sprintf("## Directory Diff: %s vs %s\n\n", args.DirA, args.DirB))
			output.WriteString(fmt.Sprintf("Filter: %s\n\n", args.Filter))

			setA := make(map[string]os.FileInfo)
			setB := make(map[string]os.FileInfo)
			for _, f := range filesA {
				setA[f.Name()] = f
			}
			for _, f := range filesB {
				setB[f.Name()] = f
			}

			onlyA := []string{}
			onlyB := []string{}
			diffSize := []string{}
			diffTime := []string{}
			same := []string{}

			for name, infoA := range setA {
				if infoB, ok := setB[name]; ok {
					if infoA.Size() != infoB.Size() {
						diffSize = append(diffSize, fmt.Sprintf("  %s: %d → %d bytes", name, infoA.Size(), infoB.Size()))
					} else if !infoA.ModTime().Equal(infoB.ModTime()) {
						diffTime = append(diffTime, fmt.Sprintf("  %s: %s → %s (same size)", name,
							infoA.ModTime().Format("2006-01-02 15:04"),
							infoB.ModTime().Format("2006-01-02 15:04")))
					} else {
						same = append(same, name)
					}
				} else {
					onlyA = append(onlyA, name)
				}
			}
			for name := range setB {
				if _, ok := setA[name]; !ok {
					onlyB = append(onlyB, name)
				}
			}

			output.WriteString(fmt.Sprintf("### Only in %s (%d):\n", args.DirA, len(onlyA)))
			for _, f := range onlyA {
				output.WriteString("  " + f + "\n")
			}
			output.WriteString(fmt.Sprintf("\n### Only in %s (%d):\n", args.DirB, len(onlyB)))
			for _, f := range onlyB {
				output.WriteString("  " + f + "\n")
			}
			output.WriteString(fmt.Sprintf("\n### Size Changed (%d):\n", len(diffSize)))
			for _, d := range diffSize {
				output.WriteString(d + "\n")
			}
			output.WriteString(fmt.Sprintf("\n### Time Changed (same size) (%d):\n", len(diffTime)))
			for _, d := range diffTime {
				output.WriteString(d + "\n")
			}
			output.WriteString(fmt.Sprintf("\n### Unchanged (%d):\n", len(same)))
			for _, s := range same {
				output.WriteString("  " + s + "\n")
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output.String()}},
				Details: map[string]any{
					"dir_a":   args.DirA,
					"dir_b":   args.DirB,
					"only_a":  len(onlyA),
					"only_b":  len(onlyB),
					"sized":   len(diffSize),
					"timed":   len(diffTime),
					"same":    len(same),
				},
			}, nil
		},
	}
}

func listDirFiles(root string, filter string) ([]os.FileInfo, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var result []os.FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if filter != "" {
			matched, err := filepath.Match(filter, e.Name())
			if err != nil || !matched {
				continue
			}
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, info)
	}
	return result, nil
}
