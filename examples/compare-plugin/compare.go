package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// providerURLFunc returns the API URL for a given provider.
// Tests override this to point at httptest servers.
var providerURLFunc = func(provider string) string {
	return fmt.Sprintf("https://api.%s.example.com/v1/chat", provider)
}

// ProviderResult holds a single provider's comparison result.
type ProviderResult struct {
	Provider string        `json:"provider"`
	Response string        `json:"response"`
	Tokens   int           `json:"tokens"`
	Latency  time.Duration `json:"latency"`
	Error    string        `json:"error,omitempty"`
}

// CompareConfig configures a comparison run.
type CompareConfig struct {
	Providers []string
	Prompt    string
	Timeout   time.Duration
}

// CompareResult holds the full comparison output.
type CompareResult struct {
	Prompt   string           `json:"prompt"`
	Results  []ProviderResult `json:"results"`
	Duration time.Duration    `json:"duration"`
}

// RunCompare executes a prompt against multiple providers in parallel.
func RunCompare(cfg CompareConfig) CompareResult {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	if len(cfg.Providers) == 0 {
		cfg.Providers = []string{"openai", "anthropic", "google"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	start := time.Now()

	var wg sync.WaitGroup
	results := make([]ProviderResult, len(cfg.Providers))

	for i, provider := range cfg.Providers {
		wg.Add(1)
		go func(idx int, prov string) {
			defer wg.Done()
			results[idx] = queryProvider(ctx, prov, cfg.Prompt)
		}(i, provider)
	}

	wg.Wait()

	return CompareResult{
		Prompt:   cfg.Prompt,
		Results:  results,
		Duration: time.Since(start),
	}
}

func queryProvider(ctx context.Context, provider, prompt string) ProviderResult {
	start := time.Now()
	result := ProviderResult{Provider: provider}

	url := providerURLFunc(provider)

	body := fmt.Sprintf(`{"messages":[{"role":"user","content":"%s"}]}`, prompt)
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(body))
	if err != nil {
		result.Error = err.Error()
		result.Latency = time.Since(start)
		return result
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Latency = time.Since(start)
		return result
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))

	var respData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	json.Unmarshal(respBody, &respData)

	if len(respData.Choices) > 0 {
		result.Response = respData.Choices[0].Message.Content
	}
	result.Tokens = respData.Usage.TotalTokens
	result.Latency = time.Since(start)
	return result
}

// FormatTable returns a plain-text comparison table.
func FormatTable(result CompareResult) string {
	var b strings.Builder
	b.WriteString("## Comparison Results\n\n")
	b.WriteString(fmt.Sprintf("Prompt: %s\n", truncate(result.Prompt, 100)))
	b.WriteString(fmt.Sprintf("Duration: %s\n\n", result.Duration.Round(time.Millisecond)))

	// Header
	b.WriteString(fmt.Sprintf("%-15s %-10s %-8s %-12s %-40s\n", "PROVIDER", "LATENCY", "TOKENS", "STATUS", "RESPONSE"))
	b.WriteString(strings.Repeat("-", 90) + "\n")

	for _, r := range result.Results {
		status := "✅ OK"
		response := truncate(r.Response, 38)
		if r.Error != "" {
			status = "❌ ERR"
			response = truncate(r.Error, 38)
		}
		b.WriteString(fmt.Sprintf("%-15s %-10s %-8d %-12s %-40s\n",
			r.Provider,
			r.Latency.Round(time.Millisecond).String(),
			r.Tokens,
			status,
			response,
		))
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
