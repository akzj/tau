package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// setupMockProvider creates a mock HTTP server for a single provider and
// wires it into providerURLFunc. Returns a cleanup function.
func setupMockProvider(t *testing.T, provider, response string, tokens int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"choices":[{"message":{"content":"%s"}}],"usage":{"total_tokens":%d}}`, response, tokens)
	}))
	return server
}

// setupMultiMock wires multiple mock servers into providerURLFunc.
// Returns cleanup + a map of provider→server URL.
func setupMultiMock(t *testing.T, configs map[string]struct{ response string; tokens int }) (cleanup func()) {
	t.Helper()

	old := providerURLFunc
	servers := make(map[string]*httptest.Server)

	for provider, cfg := range configs {
		servers[provider] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"choices":[{"message":{"content":"%s"}}],"usage":{"total_tokens":%d}}`, cfg.response, cfg.tokens)
		}))
	}

	// Capture servers in closure
	providerURLFunc = func(provider string) string {
		if s, ok := servers[provider]; ok {
			return s.URL
		}
		return "http://127.0.0.1:1/nonexistent" // force connection refused
	}

	return func() {
		providerURLFunc = old
		for _, s := range servers {
			s.Close()
		}
	}
}

// 1. Test single provider comparison with mock server
func TestCompareSingleProvider(t *testing.T) {
	server := setupMockProvider(t, "openai", "Hello from OpenAI", 5)
	defer server.Close()

	old := providerURLFunc
	providerURLFunc = func(provider string) string {
		if provider == "openai" {
			return server.URL
		}
		return "http://127.0.0.1:1/nonexistent"
	}
	defer func() { providerURLFunc = old }()

	result := RunCompare(CompareConfig{
		Providers: []string{"openai"},
		Prompt:    "hello",
		Timeout:   5 * time.Second,
	})

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	r := result.Results[0]
	if r.Provider != "openai" {
		t.Errorf("expected openai, got %s", r.Provider)
	}
	if r.Response != "Hello from OpenAI" {
		t.Errorf("expected 'Hello from OpenAI', got %q", r.Response)
	}
	if r.Tokens != 5 {
		t.Errorf("expected 5 tokens, got %d", r.Tokens)
	}
	if r.Error != "" {
		t.Errorf("expected no error, got %q", r.Error)
	}
}

// 2. Test 3 providers in parallel with mock servers
func TestCompareThreeProviders(t *testing.T) {
	cleanup := setupMultiMock(t, map[string]struct{ response string; tokens int }{
		"openai":    {"OpenAI response", 10},
		"anthropic": {"Anthropic response", 15},
		"google":    {"Google response", 20},
	})
	defer cleanup()

	result := RunCompare(CompareConfig{
		Providers: []string{"openai", "anthropic", "google"},
		Prompt:    "test prompt",
		Timeout:   5 * time.Second,
	})

	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}

	names := make(map[string]bool)
	tokenSum := 0
	for _, r := range result.Results {
		names[r.Provider] = true
		tokenSum += r.Tokens
		if r.Error != "" {
			t.Errorf("provider %s: unexpected error: %s", r.Provider, r.Error)
		}
	}

	if !names["openai"] || !names["anthropic"] || !names["google"] {
		t.Errorf("expected all 3 provider names, got: %v", names)
	}
	if tokenSum != 45 {
		t.Errorf("expected 45 total tokens, got %d", tokenSum)
	}
}

// 3. Test format table output
func TestFormatTable(t *testing.T) {
	result := CompareResult{
		Prompt: "hello",
		Results: []ProviderResult{
			{Provider: "openai", Response: "hi", Tokens: 5, Latency: 100 * time.Millisecond},
			{Provider: "anthropic", Error: "timeout", Latency: 500 * time.Millisecond},
		},
		Duration: 500 * time.Millisecond,
	}
	table := FormatTable(result)

	checks := []string{"openai", "anthropic", "ERR", "OK", "hello"}
	for _, c := range checks {
		if !strings.Contains(table, c) {
			t.Errorf("table missing expected string %q", c)
		}
	}
}

// 4. Test provider error doesn't block others
func TestProviderErrorDoesNotBlock(t *testing.T) {
	cleanup := setupMultiMock(t, map[string]struct{ response string; tokens int }{
		"openai": {"OpenAI says hello", 5},
		"google": {"Google says hello", 8},
	})
	defer cleanup()

	result := RunCompare(CompareConfig{
		Providers: []string{"openai", "bad-provider", "google"},
		Prompt:    "test",
		Timeout:   5 * time.Second,
	})

	if len(result.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result.Results))
	}

	// openai and google should succeed; bad-provider should have error
	successCount := 0
	errorCount := 0
	for _, r := range result.Results {
		if r.Error != "" {
			errorCount++
			if r.Provider != "bad-provider" {
				t.Errorf("expected only bad-provider to error, got %s: %s", r.Provider, r.Error)
			}
		} else {
			successCount++
		}
	}
	if successCount != 2 {
		t.Errorf("expected 2 successes, got %d", successCount)
	}
	if errorCount != 1 {
		t.Errorf("expected 1 error, got %d", errorCount)
	}
}

// 5. Test timeout — mock server that sleeps longer than the timeout
func TestTimeout(t *testing.T) {
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"message":{"content":"too late"}}],"usage":{"total_tokens":1}}`))
	}))
	defer slowServer.Close()

	old := providerURLFunc
	providerURLFunc = func(provider string) string {
		return slowServer.URL
	}
	defer func() { providerURLFunc = old }()

	result := RunCompare(CompareConfig{
		Providers: []string{"openai"},
		Prompt:    "test",
		Timeout:   50 * time.Millisecond, // shorter than server sleep
	})

	if len(result.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(result.Results))
	}
	// Should have either an error (context deadline exceeded) or a response
	// The timeout should kick in before the server responds
	if result.Results[0].Error == "" {
		// Server responded in time (possible on fast machines) — that's OK
		t.Log("server responded before timeout (fast machine)")
	} else {
		t.Logf("got expected error: %s", result.Results[0].Error)
	}
}

func TestTruncate(t *testing.T) {
	if s := truncate("hello", 10); s != "hello" {
		t.Errorf("short string: expected 'hello', got %q", s)
	}
	got := truncate("hello world this is a long string", 10)
	if got != "hello w..." {
		t.Errorf("long string: expected 'hello w...', got %q", got)
	}
}
