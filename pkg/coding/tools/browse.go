package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/akzj/tau/core"
	"golang.org/x/net/html"
)

// BrowseTool creates a web page reading tool.
// Fetches a URL, extracts text content, optionally filters by CSS selector (tag+class+id only).
func BrowseTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"url": {"type": "string", "description": "URL to fetch and read"},
			"extract": {"type": "string", "description": "Simple CSS selector: tag, .class, #id. e.g., 'article', '.content', '#main'. Empty = extract all text."},
			"max_chars": {"type": "integer", "description": "Maximum characters to return (default 5000)"}
		},
		"required": ["url"]
	}`)

	return core.Tool{
		Name:        "browse",
		Description: "Fetch and read a web page. Returns title and extracted text content. Supports simple CSS selectors for targeted extraction.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				URL      string `json:"url"`
				Extract  string `json:"extract"`
				MaxChars int    `json:"max_chars"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)

			if args.URL == "" {
				return core.ToolResult{}, fmt.Errorf("url required")
			}
			if args.MaxChars <= 0 {
				args.MaxChars = 5000
			}

			// Validate scheme
			if !strings.HasPrefix(args.URL, "http://") && !strings.HasPrefix(args.URL, "https://") {
				return core.ToolResult{}, fmt.Errorf("only http/https URLs allowed")
			}

			// Fetch
			client := &http.Client{Timeout: 10 * time.Second}
			req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("request: %w", err)
			}
			req.Header.Set("User-Agent", "tau/0.1")

			resp, err := client.Do(req)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("fetch: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == 404 {
				return core.ToolResult{
					Content: []core.Content{{Type: "text", Text: fmt.Sprintf("404 Not Found: %s", args.URL)}},
					Details: map[string]any{"url": args.URL, "status": 404},
				}, nil
			}
			if resp.StatusCode != 200 {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
				return core.ToolResult{}, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			}

			// Parse HTML
			doc, err := html.Parse(resp.Body)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("parse HTML: %w", err)
			}

			// Extract title
			title := extractTitle(doc)
			if title == "" {
				title = args.URL
			}

			// Extract text
			var text string
			if args.Extract != "" {
				text = extractBySelector(doc, args.Extract)
			} else {
				text = extractAllText(doc)
			}

			// Trim whitespace
			text = cleanText(text)

			// Truncate
			if len(text) > args.MaxChars {
				text = text[:args.MaxChars] + "..."
			}

			if text == "" {
				text = "(no text content extracted)"
			}

			output := fmt.Sprintf("## %s\n\n%s\n\nSource: %s", title, text, args.URL)

			return core.ToolResult{
				Content: []core.Content{{Type: "text", Text: output}},
				Details: map[string]any{"url": args.URL, "title": title, "status": resp.StatusCode, "chars": len(text)},
			}, nil
		},
	}
}

func extractTitle(n *html.Node) string {
	var title string
	var find func(*html.Node)
	find = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "title" && node.FirstChild != nil {
			title = node.FirstChild.Data
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			find(c)
		}
	}
	find(n)
	return title
}

func extractAllText(n *html.Node) string {
	var b strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		// Skip script and style
		if node.Type == html.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return b.String()
}

func extractBySelector(n *html.Node, selector string) string {
	// Simple selector parsing: "tag", ".class", "#id", or combinations like "tag.class"
	var tag, class, id string

	if strings.HasPrefix(selector, ".") {
		class = selector[1:]
	} else if strings.HasPrefix(selector, "#") {
		id = selector[1:]
	} else if idx := strings.IndexByte(selector, '.'); idx > 0 {
		tag = selector[:idx]
		class = selector[idx+1:]
	} else if idx := strings.IndexByte(selector, '#'); idx > 0 {
		tag = selector[:idx]
		id = selector[idx+1:]
	} else {
		tag = selector
	}

	var b strings.Builder
	var extract func(*html.Node)
	extract = func(node *html.Node) {
		if node.Type == html.ElementNode {
			match := true
			if tag != "" && node.Data != tag {
				match = false
			}
			if id != "" {
				found := false
				for _, attr := range node.Attr {
					if attr.Key == "id" && attr.Val == id {
						found = true
					}
				}
				if !found {
					match = false
				}
			}
			if class != "" {
				found := false
				for _, attr := range node.Attr {
					if attr.Key == "class" {
						for _, c := range strings.Fields(attr.Val) {
							if c == class {
								found = true
							}
						}
					}
				}
				if !found {
					match = false
				}
			}
			if match {
				text := extractAllText(node)
				b.WriteString(text)
				b.WriteString(" ")
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(n)
	return b.String()
}

func cleanText(s string) string {
	// Collapse whitespace
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if unicode.IsSpace(r) {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return strings.TrimSpace(b.String())
}
