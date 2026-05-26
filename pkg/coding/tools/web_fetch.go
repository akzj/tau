package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// WebFetchTool creates a web page fetching tool.
//
// Parameters:
//   url       (string, required) — the URL to fetch (http/https only)
//   max_bytes (int, optional, default 1MB) — maximum bytes to download
//
// Strips HTML tags and returns plain text content. 10-second timeout.
// User-Agent: tau/0.1. Only http/https schemes are allowed.
func WebFetchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to fetch"},
			"max_bytes": {"type": "integer", "description": "Max bytes to read (default 1MB)"}
		},
		"required": ["url"]
	}`)

	return core.Tool{
		Name:        "web_fetch",
		Description: "Fetch a web page and return its text content. Strips HTML. Max 1MB, 10s timeout.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL      string `json:"url"`
				MaxBytes int    `json:"max_bytes"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.URL == "" {
				return core.ToolResult{}, fmt.Errorf("url required")
			}
			if args.MaxBytes <= 0 {
				args.MaxBytes = 1024 * 1024
			}

			// Validate URL scheme
			parsed, err := url.Parse(args.URL)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("invalid URL: %w", err)
			}
			if parsed.Scheme != "http" && parsed.Scheme != "https" {
				return core.ToolResult{}, fmt.Errorf("only http/https allowed")
			}

			req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
			if err != nil {
				return core.ToolResult{}, err
			}
			req.Header.Set("User-Agent", "tau/0.1")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("fetch: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, int64(args.MaxBytes)))
			if err != nil {
				return core.ToolResult{}, err
			}

			// Simple HTML to text: strip tags
			text := stripHTML(string(body))
			if len(text) > 4000 {
				text = text[:4000] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: text}},
				Details: map[string]any{
					"url":          args.URL,
					"status":       float64(resp.StatusCode),
					"content_type": resp.Header.Get("Content-Type"),
					"size":         len(body),
				},
			}, nil
		},
	}
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

func stripHTML(s string) string {
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = strings.ReplaceAll(s, "&amp;", "&")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", "\"")
	s = multiSpaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
