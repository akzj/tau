package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/akzj/tau/core"
)

// WebSearchTool creates a DuckDuckGo web search tool (zero API keys required).
//
// Parameters:
//   query (string, required) — the search query string
//
// Returns: abstract/answer text, source URL, and up to 5 related topics.
// Uses the free DuckDuckGo Instant Answer API. No authentication required.
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

			if core.DefaultSearchProvider == nil {
				return core.ToolResult{}, fmt.Errorf("search not available")
			}

			results, err := core.DefaultSearchProvider.Search(ctx, args.Query)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("search: %w", err)
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
