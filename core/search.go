package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchResult is a single web search result item.
type WebSearchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

// SearchProvider is a pluggable search backend.
type SearchProvider interface {
	Search(ctx context.Context, query string) ([]WebSearchResult, error)
	Name() string
}

// DefaultSearchProvider is the global search backend. Set to nil to disable.
var DefaultSearchProvider SearchProvider

func init() {
	DefaultSearchProvider = NewDuckDuckGoSearch()
}

// DuckDuckGoSearch implements SearchProvider using DuckDuckGo Instant Answer API.
// Zero API keys required.
type DuckDuckGoSearch struct {
	client *http.Client
}

// NewDuckDuckGoSearch creates a DuckDuckGo search provider.
func NewDuckDuckGoSearch() *DuckDuckGoSearch {
	return &DuckDuckGoSearch{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Name returns "duckduckgo".
func (d *DuckDuckGoSearch) Name() string { return "duckduckgo" }

// Search performs a web search via DuckDuckGo Instant Answer API.
func (d *DuckDuckGoSearch) Search(ctx context.Context, query string) ([]WebSearchResult, error) {
	apiURL := "https://api.duckduckgo.com/?q=" + url.QueryEscape(query) + "&format=json&no_html=1&skip_disambig=1"

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("search request: %w", err)
	}
	req.Header.Set("User-Agent", "tau/0.1")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return nil, fmt.Errorf("search read: %w", err)
	}

	var ddgResp struct {
		Abstract    string `json:"Abstract"`
		AbstractURL string `json:"AbstractURL"`
		Heading     string `json:"Heading"`
		Answer      string `json:"Answer"`
		Results     []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"Results"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
	}
	json.Unmarshal(body, &ddgResp)

	var results []WebSearchResult

	if ddgResp.Answer != "" {
		results = append(results, WebSearchResult{
			Title:   ddgResp.Heading,
			URL:     ddgResp.AbstractURL,
			Snippet: ddgResp.Answer,
		})
	}

	if ddgResp.Abstract != "" && len(results) == 0 {
		results = append(results, WebSearchResult{
			Title:   query,
			URL:     ddgResp.AbstractURL,
			Snippet: ddgResp.Abstract,
		})
	}

	for _, t := range ddgResp.RelatedTopics {
		if len(results) >= 5 {
			break
		}
		if t.Text != "" {
			results = append(results, WebSearchResult{
				Title:   extractSearchTitle(t.Text),
				URL:     t.FirstURL,
				Snippet: t.Text,
			})
		}
	}

	for _, r := range ddgResp.Results {
		if len(results) >= 5 {
			break
		}
		if r.Text != "" {
			results = append(results, WebSearchResult{
				Title:   extractSearchTitle(r.Text),
				URL:     r.FirstURL,
				Snippet: r.Text,
			})
		}
	}

	return results, nil
}

func extractSearchTitle(text string) string {
	if idx := strings.IndexByte(text, '.'); idx > 0 && idx < 60 {
		return text[:idx+1]
	}
	if len(text) > 60 {
		return text[:57] + "..."
	}
	return text
}
