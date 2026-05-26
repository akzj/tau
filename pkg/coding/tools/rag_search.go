package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/akzj/tau/core"
)

// RAGSearchTool creates a semantic code search tool using TF-IDF.
func RAGSearchTool() core.Tool {
	schema := Schema{Raw: json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Natural language query describing what you're looking for"},
			"top_k": {"type": "integer", "description": "Number of results to return (default 10)"}
		},
		"required": ["query"]
	}`)}

	var idx *core.RAGIndex // cached index

	return core.Tool{
		Name:        "rag_search",
		Description: "Semantic code search using TF-IDF. Finds relevant files by meaning, not exact text match.",
		Schema:      schema,
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Query string `json:"query"`
				TopK  int    `json:"top_k"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Query == "" {
				return core.ToolResult{}, fmt.Errorf("rag_search: query required")
			}
			if args.TopK <= 0 {
				args.TopK = 10
			}

			// Build or reuse index
			if idx == nil {
				idx = core.NewRAGIndex()
				if err := idx.IndexWorkspace(WorkspaceRoot, []string{".go", ".py", ".js", ".ts", ".md", ".yaml", ".json"}); err != nil {
					return core.ToolResult{}, fmt.Errorf("rag_search: index: %w", err)
				}
			}

			results := idx.Search(args.Query, args.TopK)
			if len(results) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: "No matching files found."}},
				}, nil
			}

			var b strings.Builder
			b.WriteString(fmt.Sprintf("## Semantic Search: %q\n\n", args.Query))
			for i, r := range results {
				b.WriteString(fmt.Sprintf("%d. **%s** (score: %.3f)\n", i+1, r.Path, r.Score))
				if r.Snippet != "" {
					b.WriteString(fmt.Sprintf("   `%s`\n", r.Snippet))
				}
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: b.String()}},
				Details: map[string]any{"query": args.Query, "results": len(results), "top_k": args.TopK},
			}, nil
		},
	}
}