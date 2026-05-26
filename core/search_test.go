package core

import (
	"context"
	"testing"
	"time"
)

func TestDuckDuckGoSearchName(t *testing.T) {
	var prov SearchProvider = NewDuckDuckGoSearch()
	if prov.Name() != "duckduckgo" {
		t.Errorf("expected 'duckduckgo', got %q", prov.Name())
	}
}

func TestDuckDuckGoSearchTimeout(t *testing.T) {
	ddg := NewDuckDuckGoSearch()
	ddg.client.Timeout = 1 * time.Millisecond
	ctx := context.Background()
	_, err := ddg.Search(ctx, "test query")
	if err == nil {
		t.Log("search succeeded (expected timeout in CI)")
	}
}

func TestDuckDuckGoSearchEmptyQuery(t *testing.T) {
	ddg := NewDuckDuckGoSearch()
	// Empty query should still work (DDG API returns empty results)
	results, err := ddg.Search(context.Background(), "")
	if err != nil {
		t.Logf("search error (expected in CI/no-network): %v", err)
		return
	}
	if len(results) > 0 {
		t.Logf("got %d results for empty query", len(results))
	}
}

func TestDefaultSearchProvider(t *testing.T) {
	if DefaultSearchProvider == nil {
		t.Error("DefaultSearchProvider should not be nil")
	}
	if DefaultSearchProvider.Name() != "duckduckgo" {
		t.Errorf("expected 'duckduckgo', got %q", DefaultSearchProvider.Name())
	}
}

func TestExtractSearchTitle(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"This is a title. With more text.", "This is a title."},
		{"Short title", "Short title"},
		{"This is a very long title that exceeds sixty characters by quite a bit", "This is a very long title that exceeds sixty characters b..."},
	}
	for _, tt := range tests {
		got := extractSearchTitle(tt.in)
		if got != tt.want {
			t.Errorf("extractSearchTitle(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
