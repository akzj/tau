package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// GitHubSearchTool creates a GitHub code/repo search tool.
//
// Parameters:
//
//	query    (string, required) — search query
//	type     (string, optional, default: code) — search type: code | repo | issue
//	per_page (int, optional, default: 30, max 100) — results per page
//
// Requires GITHUB_TOKEN environment variable.
func GitHubSearchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query (see GitHub search syntax)"},
			"type": {"type": "string", "description": "Search type: code, repo, issue (default: code)"},
			"per_page": {"type": "integer", "description": "Results per page (default 30, max 100)"}
		},
		"required": ["query"]
	}`)

	return core.Tool{
		Name:        "github_search",
		Description: "Search GitHub: code, repos, issues. Requires GITHUB_TOKEN env var.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Query   string `json:"query"`
				Type    string `json:"type"`
				PerPage int    `json:"per_page"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Query == "" {
				return core.ToolResult{}, fmt.Errorf("query required")
			}
			if args.Type == "" {
				args.Type = "code"
			}
			if args.PerPage <= 0 {
				args.PerPage = 30
			}
			if args.PerPage > 100 {
				args.PerPage = 100
			}

			token := os.Getenv("GITHUB_TOKEN")
			if token == "" {
				return core.ToolResult{}, fmt.Errorf("GITHUB_TOKEN not set")
			}

			// Determine API endpoint based on type
			var apiURL string
			switch args.Type {
			case "code":
				apiURL = "https://api.github.com/search/code"
			case "repo":
				apiURL = "https://api.github.com/search/repositories"
			case "issue":
				apiURL = "https://api.github.com/search/issues"
			default:
				return core.ToolResult{}, fmt.Errorf("unknown search type: %s (use code/repo/issue)", args.Type)
			}

			fullURL := fmt.Sprintf("%s?q=%s&per_page=%d", apiURL, url.QueryEscape(args.Query), args.PerPage)
			req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
			if err != nil {
				return core.ToolResult{}, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Accept", "application/vnd.github+json")
			req.Header.Set("User-Agent", "tau/0.1")

			client := &http.Client{Timeout: 15 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("github API: %w", err)
			}
			defer resp.Body.Close()

			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				return core.ToolResult{}, fmt.Errorf("github API returned %d: %s", resp.StatusCode, string(body))
			}

			var result struct {
				TotalCount int `json:"total_count"`
				Items      []struct {
					Name       string `json:"name"`
					Path       string `json:"path"`
					HTMLURL    string `json:"html_url"`
					Repository struct {
						FullName string `json:"full_name"`
					} `json:"repository"`
				} `json:"items"`
			}
			if err := json.Unmarshal(body, &result); err != nil {
				return core.ToolResult{}, fmt.Errorf("parse response: %w", err)
			}

			output := fmt.Sprintf("## GitHub Search: %s (type: %s)\nTotal: %d\n\n", args.Query, args.Type, result.TotalCount)
			if len(result.Items) == 0 {
				output += "No results found."
			} else {
				for i, item := range result.Items {
					switch args.Type {
					case "repo":
						output += fmt.Sprintf("%d. **%s** — %s\n", i+1, item.Repository.FullName, item.HTMLURL)
					default:
						output += fmt.Sprintf("%d. **%s** (%s) — %s\n", i+1, item.Name, item.Repository.FullName, item.HTMLURL)
					}
				}
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"query": args.Query, "type": args.Type, "total": result.TotalCount, "returned": len(result.Items)},
			}, nil
		},
	}
}
