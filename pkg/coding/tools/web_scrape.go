package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/akzj/tau/core"
	"golang.org/x/net/html"
)

// WebScrapeTool creates a web scraping tool with CSS/XPath selection.
//
// Parameters:
//
//	url       (string, required) — the URL to fetch (http/https only)
//	selector  (string, required) — CSS selector to match elements
//	attribute (string, optional) — extract this attribute value instead of text content
//
// Fetches HTML from url, parses with golang.org/x/net/html, and extracts text
// from all elements matching the CSS selector. Supports basic CSS selectors:
// tag, .class, #id, tag.class, and simple descendant combinators.
func WebScrapeTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to scrape (http/https)"},
			"selector": {"type": "string", "description": "CSS selector (tag, .class, #id, tag.class, div p)"},
			"attribute": {"type": "string", "description": "Extract this attribute instead of text content (e.g., 'href', 'src')"}
		},
		"required": ["url", "selector"]
	}`)

	return core.Tool{
		Name:        "web_scrape",
		Description: "Fetch a web page and extract text from elements matching a CSS selector. Uses net/html for parsing.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL       string `json:"url"`
				Selector  string `json:"selector"`
				Attribute string `json:"attribute"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.URL == "" {
				return core.ToolResult{}, fmt.Errorf("url required")
			}
			if args.Selector == "" {
				return core.ToolResult{}, fmt.Errorf("selector required")
			}

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

			body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
			if err != nil {
				return core.ToolResult{}, err
			}

			doc, err := html.Parse(strings.NewReader(string(body)))
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("parse HTML: %w", err)
			}

			matches := findMatching(doc, args.Selector)
			if len(matches) == 0 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("No elements matched selector: %s", args.Selector)}},
					Details: map[string]any{"url": args.URL, "matches": 0},
				}, nil
			}

			var lines []string
			for _, n := range matches {
				if args.Attribute != "" {
					for _, a := range n.Attr {
						if a.Key == args.Attribute {
							lines = append(lines, a.Val)
							break
						}
					}
				} else {
					t := extractText(n)
					if t != "" {
						lines = append(lines, t)
					}
				}
			}

			if len(lines) > 50 {
				lines = lines[:50]
			}

			output := fmt.Sprintf("## Scrape: %s\nSelector: %s\nMatches: %d\n\n", args.URL, args.Selector, len(matches))
			for i, line := range lines {
				output += fmt.Sprintf("%d. %s\n", i+1, line)
			}

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"url": args.URL, "selector": args.Selector, "matches": len(matches)},
			}, nil
		},
	}
}

// parseCSSSelector splits a simple CSS selector into tag, class, and id parts.
func parseCSSSelector(sel string) (tag, class, id string) {
	// Handle #id
	if idx := strings.Index(sel, "#"); idx >= 0 {
		idPart := sel[idx+1:]
		if dotIdx := strings.Index(idPart, "."); dotIdx >= 0 {
			id = idPart[:dotIdx]
		} else {
			id = idPart
		}
		if idx > 0 {
			tag = sel[:idx]
		}
		return
	}
	// Handle .class
	if idx := strings.Index(sel, "."); idx >= 0 {
		if idx > 0 {
			tag = sel[:idx]
		}
		class = sel[idx+1:]
		return
	}
	// Plain tag
	tag = sel
	return
}

// findMatching traverses the HTML tree and returns nodes matching the CSS selector.
func findMatching(n *html.Node, selector string) []*html.Node {
	// Support descendant combinator: "div p" matches p inside div
	if strings.Contains(selector, " ") {
		parts := strings.Fields(selector)
		if len(parts) == 2 {
			ancestors := findMatching(n, parts[0])
			var results []*html.Node
			for _, anc := range ancestors {
				results = append(results, findDescendantsByTag(anc, parts[1])...)
			}
			return results
		}
	}

	tag, class, id := parseCSSSelector(selector)
	var results []*html.Node
	findAll(n, func(node *html.Node) bool {
		if node.Type != html.ElementNode {
			return true
		}
		if tag != "" && node.Data != tag {
			return true
		}
		if class != "" && !hasClass(node, class) {
			return true
		}
		if id != "" && getAttr(node, "id") != id {
			return true
		}
		results = append(results, node)
		return true
	})
	return results
}

func findDescendantsByTag(n *html.Node, tag string) []*html.Node {
	var results []*html.Node
	findAllDescendants(n, func(node *html.Node) bool {
		if node.Type == html.ElementNode && node.Data == tag {
			results = append(results, node)
		}
		return true
	})
	return results
}

func findAll(n *html.Node, fn func(*html.Node) bool) {
	if !fn(n) {
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findAll(c, fn)
	}
}

func findAllDescendants(n *html.Node, fn func(*html.Node) bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if !fn(c) {
			return
		}
		findAllDescendants(c, fn)
	}
}

func hasClass(n *html.Node, class string) bool {
	cls := getAttr(n, "class")
	if cls == "" {
		return false
	}
	for _, c := range strings.Fields(cls) {
		if c == class {
			return true
		}
	}
	return false
}

func getAttr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func extractText(n *html.Node) string {
	var parts []string
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			t := strings.TrimSpace(node.Data)
			if t != "" {
				parts = append(parts, t)
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(parts, " ")
}
