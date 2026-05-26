package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/akzj/tau/core"
)

// WebSearchTool creates a web search tool using DuckDuckGo Instant Answer API.
func WebSearchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"}
		},
		"required": ["query"]
	}`)

	return core.Tool{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo. Returns abstract, URL, and related topics.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct{ Query string `json:"query"` }
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.ToolResult{}, err
			}
			if args.Query == "" {
				return core.ToolResult{}, fmt.Errorf("query required")
			}

			// DuckDuckGo Instant Answer API
			apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(args.Query) + "&format=json&no_html=1&skip_disambig=1"

			req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
			if err != nil {
				return core.ToolResult{}, err
			}
			req.Header.Set("User-Agent", "tau/0.1")

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("search: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
			if err != nil {
				return core.ToolResult{}, err
			}

			var result struct {
				Abstract      string `json:"Abstract"`
				AbstractURL   string `json:"AbstractURL"`
				Heading       string `json:"Heading"`
				Answer        string `json:"Answer"`
				RelatedTopics []struct {
					Text string `json:"Text"`
				} `json:"RelatedTopics"`
			}
			json.Unmarshal(body, &result)

			output := fmt.Sprintf("## %s\n\n", args.Query)
			if result.Answer != "" {
				output += fmt.Sprintf("**Answer**: %s\n\n", result.Answer)
			}
			if result.Abstract != "" {
				output += fmt.Sprintf("%s\n", result.Abstract)
				if result.AbstractURL != "" {
					output += fmt.Sprintf("\nSource: %s\n", result.AbstractURL)
				}
			}
			if len(result.RelatedTopics) > 0 {
				output += "\n### Related\n"
				for i, t := range result.RelatedTopics {
					if i >= 5 {
						break
					}
					output += fmt.Sprintf("- %s\n", t.Text)
				}
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"query": args.Query, "has_answer": result.Answer != ""},
			}, nil
		},
	}
}
