package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/akzj/tau/core"
)

// WebDownloadTool creates an HTTP file download tool with resume support.
//
// Parameters:
//
//	url       (string, required) — URL to download
//	dest      (string, required) — destination file path
//	resume    (bool, optional) — resume partial download
func WebDownloadTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to download"},
			"dest": {"type": "string", "description": "Destination file path"},
			"resume": {"type": "boolean", "description": "Resume partial download if dest exists"}
		},
		"required": ["url", "dest"]
	}`)

	return core.Tool{
		Name:        "web_download",
		Description: "Download a file from URL with optional resume support. Supports HTTP range requests for partial downloads.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL    string `json:"url"`
				Dest   string `json:"dest"`
				Resume bool   `json:"resume"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.URL == "" || args.Dest == "" {
				return core.ToolResult{}, fmt.Errorf("url and dest required")
			}

			// Resolve dest path
			dest := args.Dest
			if !filepath.IsAbs(dest) {
				dest = filepath.Join(WorkspaceRoot, dest)
			}

			// Check resume
			var offset int64
			if args.Resume {
				if fi, err := os.Stat(dest); err == nil {
					offset = fi.Size()
				}
			}

			client := &http.Client{Timeout: 300 * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("create request: %w", err)
			}

			if offset > 0 {
				req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
			}

			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("download: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
				return core.ToolResult{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
			}

			var f *os.File
			if offset > 0 && resp.StatusCode == http.StatusPartialContent {
				f, err = os.OpenFile(dest, os.O_APPEND|os.O_WRONLY, 0644)
			} else {
				f, err = os.Create(dest)
			}
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("open dest: %w", err)
			}
			defer f.Close()

			written, err := io.Copy(f, resp.Body)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("write: %w", err)
			}

			contentLen := resp.Header.Get("Content-Length")
			sizeMsg := fmt.Sprintf("%d bytes written", written)
			if contentLen != "" {
				if cl, e := strconv.ParseInt(contentLen, 10, 64); e == nil {
					sizeMsg = fmt.Sprintf("%d/%d bytes", written+offset, cl+offset)
				}
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: fmt.Sprintf("Downloaded: %s → %s (%s)", args.URL, dest, sizeMsg)}},
				Details: map[string]any{"url": args.URL, "dest": dest, "bytes_written": written, "success": true},
			}, nil
		},
	}
}
