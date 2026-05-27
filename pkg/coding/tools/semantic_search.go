package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/akzj/tau/core"
)

// SemanticSearchTool creates an enhanced semantic code search tool.
// Extends rag_search with snippet extraction, highlighting, git history search,
// and configurable scopes.
func SemanticSearchTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {"type": "string", "description": "Natural language or code query to search for"},
			"scope": {"type": "string", "description": "Comma-separated scopes: code, docs, git (default: code)"},
			"top_k": {"type": "integer", "description": "Number of results to return (default: 10)"},
			"min_score": {"type": "number", "description": "Minimum relevance score 0-1 (default: 0.05)"},
			"highlight": {"type": "boolean", "description": "Whether to bold matching terms (default: true)"}
		},
		"required": ["query"]
	}`)

	var idx *core.RAGIndex

	return core.Tool{
		Name:        "semantic_search",
		Description: "Semantic code search using TF-IDF with snippet extraction, highlighting, and git history search.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Query     string  `json:"query"`
				Scope     string  `json:"scope"`
				TopK      int     `json:"top_k"`
				MinScore  float64 `json:"min_score"`
				Highlight bool    `json:"highlight"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.Query == "" {
				return core.ToolResult{}, fmt.Errorf("semantic_search: query required")
			}
			if args.TopK <= 0 {
				args.TopK = 10
			}
			if args.MinScore <= 0 {
				args.MinScore = 0.05
			}
			if args.Scope == "" {
				args.Scope = "code"
			}

			highlight := true
			if rawStr, _ := json.Marshal(params); rawStr != nil {
				var check struct {
					Highlight *bool `json:"highlight"`
				}
				json.Unmarshal(rawStr, &check)
				if check.Highlight != nil && !*check.Highlight {
					highlight = false
				}
			}

			scopes := parseScopes(args.Scope)

			var b strings.Builder
			b.WriteString(fmt.Sprintf("## Semantic Search: %q\n\n", args.Query))

			totalResults := 0

			if scopes["code"] || scopes["docs"] {
				extensions := []string{}
				if scopes["code"] {
					extensions = append(extensions, ".go", ".py", ".js", ".ts", ".rs", ".java", ".c", ".h", ".cpp", ".hpp")
				}
				if scopes["docs"] {
					extensions = append(extensions, ".md", ".txt", ".rst", ".yaml", ".yml", ".json", ".toml")
				}

				if idx == nil {
					idx = core.NewRAGIndex()
					if err := idx.IndexWorkspace(WorkspaceRoot, extensions); err != nil {
						return core.ToolResult{}, fmt.Errorf("semantic_search: index: %w", err)
					}
				}

				results := idx.Search(args.Query, args.TopK)

				if len(results) > 0 {
					filtered := []core.SearchResult{}
					for _, r := range results {
						if r.Score >= args.MinScore {
							filtered = append(filtered, r)
						}
					}
					if len(filtered) > args.TopK {
						filtered = filtered[:args.TopK]
					}

					if scopes["code"] && !scopes["docs"] {
						b.WriteString("### Code Results\n")
					} else if scopes["docs"] && !scopes["code"] {
						b.WriteString("### Document Results\n")
					} else {
						b.WriteString("### Results\n")
					}

					for i, r := range filtered {
						b.WriteString(fmt.Sprintf("%d. **%s** (score: %.3f)\n", i+1, r.Path, r.Score))
						snippet := extractSnippet(WorkspaceRoot, r.Path, args.Query, 5)
						if snippet != "" {
							if highlight {
								snippet = highlightTerms(snippet, args.Query)
							}
							b.WriteString("   ```\n")
							for _, line := range strings.Split(snippet, "\n") {
								b.WriteString(fmt.Sprintf("   %s\n", line))
							}
							b.WriteString("   ```\n")
						}
						totalResults++
					}
					b.WriteString("\n")
				}
			}

			if scopes["git"] {
				gitResults := searchGitHistory(ctx, args.Query, args.TopK)
				if len(gitResults) > 0 {
					b.WriteString("### Git History\n")
					for _, gr := range gitResults {
						line := gr
						if highlight {
							line = highlightTerms(line, args.Query)
						}
						b.WriteString(fmt.Sprintf("- %s\n", line))
						totalResults++
					}
					b.WriteString("\n")
				}
			}

			if totalResults == 0 {
				b.WriteString("No results found.\n")
			}

			b.WriteString(fmt.Sprintf("### Summary\n%d results found (top_k: %d, min_score: %.2f)\n",
				totalResults, args.TopK, args.MinScore))

			output := b.String()
			if len(output) > OutputCap {
				output = output[:OutputCap] + "\n... (truncated)"
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{
					"query":   args.Query,
					"results": totalResults,
					"top_k":   args.TopK,
					"scope":   args.Scope,
				},
			}, nil
		},
	}
}

func parseScopes(scope string) map[string]bool {
	result := map[string]bool{}
	for _, s := range strings.Split(scope, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result[s] = true
		}
	}
	return result
}

func extractSnippet(workspaceRoot, path, query string, contextLines int) string {
	fullPath := filepath.Join(workspaceRoot, path)
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return ""
	}

	contents := string(content)
	lower := strings.ToLower(contents)

	bestPos := -1
	for _, token := range tokenizeQuery(query) {
		pos := strings.Index(lower, strings.ToLower(token))
		if pos >= 0 {
			bestPos = pos
			break
		}
	}
	if bestPos < 0 {
		lineEnd := 0
		for i := 0; i < contextLines && lineEnd < len(contents); i++ {
			if idx := strings.IndexByte(contents[lineEnd:], '\n'); idx >= 0 {
				lineEnd += idx + 1
			} else {
				lineEnd = len(contents)
				break
			}
		}
		if lineEnd > 200 {
			lineEnd = 200
		}
		return contents[:lineEnd]
	}

	start := bestPos
	for start > 0 && contents[start-1] != '\n' {
		start--
	}

	prefix := start
	for i := 0; i < contextLines && prefix > 0; i++ {
		prev := strings.LastIndexByte(contents[:prefix-1], '\n')
		if prev < 0 {
			prefix = 0
			break
		}
		prefix = prev + 1
	}

	end := bestPos
	for end < len(contents) && contents[end] != '\n' {
		end++
	}
	for i := 0; i < contextLines && end < len(contents); i++ {
		end++
		if next := strings.IndexByte(contents[end:], '\n'); next >= 0 {
			end += next
		} else {
			end = len(contents)
			break
		}
	}

	snippet := contents[prefix:end]
	snippet = strings.TrimRight(snippet, "\n")
	return snippet
}

func highlightTerms(text, query string) string {
	tokens := tokenizeQuery(query)
	result := text
	for _, token := range tokens {
		if len(token) < 2 {
			continue
		}
		re, err := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(token) + `\b`)
		if err != nil {
			continue
		}
		result = re.ReplaceAllStringFunc(result, func(m string) string {
			return "**" + m + "**"
		})
	}
	return result
}

func tokenizeQuery(query string) []string {
	re := regexp.MustCompile(`[a-zA-Z0-9_]+`)
	matches := re.FindAllString(query, -1)

	stopWords := map[string]bool{
		"the": true, "is": true, "at": true, "which": true, "on": true, "a": true, "an": true,
		"and": true, "or": true, "not": true, "but": true, "in": true, "to": true, "for": true,
		"of": true, "with": true, "from": true, "by": true, "as": true, "be": true, "are": true,
		"was": true, "were": true, "if": true, "else": true, "when": true, "where": true, "how": true,
		"what": true, "this": true, "that": true, "it": true, "its": true, "do": true, "does": true,
	}

	var tokens []string
	for _, m := range matches {
		lower := strings.ToLower(m)
		if len(lower) < 2 || stopWords[lower] {
			continue
		}
		tokens = append(tokens, m)
	}
	return tokens
}

func searchGitHistory(ctx context.Context, query string, topK int) []string {
	cmd := exec.CommandContext(ctx, "git", "log", "--oneline", "--all",
		fmt.Sprintf("--grep=%s", query), fmt.Sprintf("-n=%d", topK))
	cmd.Dir = WorkspaceRoot
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var result []string
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}
