package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/akzj/tau/core"
)

// rate limiter for web_search (1 req/s)
var (
	webSearchLastCall time.Time
	webSearchMu       sync.Mutex
)

// WebSearchTool creates a DuckDuckGo web search tool (zero API keys required).
//
// Parameters:
//
//	query      (string, required)  — the search query string
//	max_results (int, optional, default 10, max 20) — max results to return
//
// Returns: title, URL, and snippet for each result.
// Auto-rate-limits to 1 request per second.
// Uses the free DuckDuckGo Instant Answer API. No authentication required.
func WebSearchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Search query"},
			"max_results": {"type": "integer", "description": "Max results to return (default 10, max 20)"}
		},
		"required": ["query"]
	}`)

	return core.Tool{
		Name:        "web_search",
		Description: "Search the web using DuckDuckGo. Returns title, URL, and snippet. Auto-rate-limited to 1 req/s.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Query      string `json:"query"`
				MaxResults int    `json:"max_results"`
			}
			raw, _ := json.Marshal(params)
			if err := json.Unmarshal(raw, &args); err != nil {
				return core.ToolResult{}, err
			}
			if args.Query == "" {
				return core.ToolResult{}, fmt.Errorf("query required")
			}
			if args.MaxResults <= 0 {
				args.MaxResults = 10
			}
			if args.MaxResults > 20 {
				args.MaxResults = 20
			}

			// Rate limiting: 1 req/s
			webSearchMu.Lock()
			elapsed := time.Since(webSearchLastCall)
			if elapsed < time.Second {
				wait := time.Second - elapsed
				webSearchMu.Unlock()
				select {
				case <-time.After(wait):
				case <-ctx.Done():
					return core.ToolResult{}, ctx.Err()
				}
				webSearchMu.Lock()
			}
			webSearchLastCall = time.Now()
			webSearchMu.Unlock()

			if core.DefaultSearchProvider == nil {
				return core.ToolResult{}, fmt.Errorf("search not available")
			}

			results, err := core.DefaultSearchProvider.Search(ctx, args.Query)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("search: %w", err)
			}

			if len(results) > args.MaxResults {
				results = results[:args.MaxResults]
			}

			output := fmt.Sprintf("## Web Search: %s\n\n", args.Query)
			if len(results) == 0 {
				output += "No results found."
			} else {
				for i, r := range results {
					output += fmt.Sprintf("%d. **%s**\n   %s\n   %s\n\n", i+1, r.Title, r.Snippet, r.URL)
				}
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"query": args.Query, "results": len(results)},
			}, nil
		},
	}
}
