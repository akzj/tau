package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// FileArchiveTool creates archive create/extract tool.
//
// Parameters:
//
//	action (string, required) — create | extract
//	source (string, required) — source path (file/dir for create, archive for extract)
//	dest   (string, required) — destination path
//	format (string, optional) — tar | tar.gz | zip (auto-detected from extension if omitted)
func FileArchiveTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "description": "create or extract"},
			"source": {"type": "string", "description": "Source path (file/dir for create, archive for extract)"},
			"dest": {"type": "string", "description": "Destination path"},
			"format": {"type": "string", "description": "Archive format: tar, tar.gz, zip (auto-detected from extension if omitted)"}
		},
		"required": ["action", "source", "dest"]
	}`)

	return core.Tool{
		Name:        "file_archive",
		Description: "Create or extract tar, tar.gz, and zip archives. Format auto-detected from file extension.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Action string `json:"action"`
				Source string `json:"source"`
				Dest   string `json:"dest"`
				Format string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Action == "" || args.Source == "" || args.Dest == "" {
				return core.ToolResult{}, fmt.Errorf("action, source, and dest required")
			}

			// Resolve paths
			src := args.Source
			if !filepath.IsAbs(src) {
				src = filepath.Join(WorkspaceRoot, src)
			}
			dst := args.Dest
			if !filepath.IsAbs(dst) {
				dst = filepath.Join(WorkspaceRoot, dst)
			}

			format := args.Format
			if format == "" {
				format = detectFormat(dst)
			}

			switch args.Action {
			case "create":
				return createArchive(src, dst, format)
			case "extract":
				return extractArchive(src, dst, format)
			default:
				return core.ToolResult{}, fmt.Errorf("unknown action: %s (use create/extract)", args.Action)
			}
		},
	}
}

func detectFormat(path string) string {
	path = strings.ToLower(path)
	if strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz") {
		return "tar.gz"
	}
	if strings.HasSuffix(path, ".tar") {
		return "tar"
	}
	if strings.HasSuffix(path, ".zip") {
		return "zip"
	}
	return "tar.gz" // default
}

func createArchive(src, dst, format string) (core.ToolResult, error) {
	switch format {
	case "zip":
		return createZip(src, dst)
	case "tar":
		return createTar(src, dst, false)
	case "tar.gz":
		return createTar(src, dst, true)
	default:
		return core.ToolResult{}, fmt.Errorf("unsupported format: %s", format)
	}
}

func extractArchive(src, dst, format string) (core.ToolResult, error) {
	switch format {
	case "zip":
		return extractZip(src, dst)
	case "tar":
		return extractTar(src, dst, false)
	case "tar.gz":
		return extractTar(src, dst, true)
	default:
		return core.ToolResult{}, fmt.Errorf("unsupported format: %s", format)
	}
}

func createTar(src, dst string, gzipFlag bool) (core.ToolResult, error) {
	f, err := os.Create(dst)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	var w *tar.Writer
	if gzipFlag {
		gw := gzip.NewWriter(f)
		defer gw.Close()
		w = tar.NewWriter(gw)
	} else {
		w = tar.NewWriter(f)
	}
	defer w.Close()

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Archive created: %s (tar%s)", dst, map[bool]string{true: ".gz", false: ""}[gzipFlag])}},
		Details: map[string]any{"source": src, "dest": dst, "format": map[bool]string{true: "tar.gz", false: "tar"}[gzipFlag], "success": true},
	}, filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		header.Name = rel
		if err := w.WriteHeader(header); err != nil {
			return err
		}
		if !info.IsDir() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			_, err = w.Write(data)
			return err
		}
		return nil
	})
}

func extractTar(src, dst string, gzipFlag bool) (core.ToolResult, error) {
	f, err := os.Open(src)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var r *tar.Reader
	if gzipFlag {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return core.ToolResult{}, fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		r = tar.NewReader(gz)
	} else {
		r = tar.NewReader(f)
	}

	count := 0
	for {
		header, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return core.ToolResult{}, fmt.Errorf("tar read: %w", err)
		}

		target := filepath.Join(dst, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, _ := os.Create(target)
			io.Copy(out, r)
			out.Close()
		}
		count++
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Extracted %d entries from %s to %s", count, src, dst)}},
		Details: map[string]any{"source": src, "dest": dst, "entries": count, "success": true},
	}, nil
}

func createZip(src, dst string) (core.ToolResult, error) {
	f, err := os.Create(dst)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Archive created: %s (zip)", dst)}},
		Details: map[string]any{"source": src, "dest": dst, "format": "zip", "success": true},
	}, filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if rel == "." {
			return nil
		}
		if info.IsDir() {
			rel += "/"
		}
		header, _ := zip.FileInfoHeader(info)
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := w.CreateHeader(header)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			data, _ := os.ReadFile(path)
			_, err = writer.Write(data)
		}
		return err
	})
}

func extractZip(src, dst string) (core.ToolResult, error) {
	r, err := zip.OpenReader(src)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	for _, f := range r.File {
		target := filepath.Join(dst, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		rc, _ := f.Open()
		out, _ := os.Create(target)
		io.Copy(out, rc)
		rc.Close()
		out.Close()
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Extracted %d entries from %s to %s", len(r.File), src, dst)}},
		Details: map[string]any{"source": src, "dest": dst, "entries": len(r.File), "success": true},
	}, nil
}
