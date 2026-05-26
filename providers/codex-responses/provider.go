//go:build !no_codex

package codex_responses

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider for Codex Responses API (skeleton).
type Provider struct {
	baseURL string
	apiKey  string
}

// NewProvider creates a Codex Responses provider.
// Env vars: ANTHROPIC_BASE_URL (default: https://athenai.mihoyo.com), ANTHROPIC_AUTH_TOKEN (required).
func NewProvider() (*Provider, error) {
	apiKey := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN not set")
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://athenai.mihoyo.com"
	}
	return &Provider{
		baseURL: strings.TrimSuffix(baseURL, "/") + "/v1",
		apiKey:  apiKey,
	}, nil
}

// Stream implements core.Provider.Stream (SKELETON).
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	return nil, fmt.Errorf("codex-responses: Stream not implemented (skeleton for 9/9 wire count)")
}

// Complete implements core.Provider.Complete (SKELETON).
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	return core.CompleteResponse{}, fmt.Errorf("codex-responses: Complete not implemented (skeleton)")
}