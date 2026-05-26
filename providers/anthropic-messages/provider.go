package anthropic_messages

import (
	"context"
	"fmt"
	"os"

	"github.com/akzj/tau/core"
)

// AnthropicMessagesProvider implements core.Provider for Anthropic Messages API.
type AnthropicMessagesProvider struct {
	baseURL string
	apiKey  string
}

// NewAnthropicMessagesProvider creates a provider for Anthropic Messages wire.
// Uses ANTHROPIC_AUTH_TOKEN and ANTHROPIC_BASE_URL env vars (same as OpenAI gateway).
func NewAnthropicMessagesProvider() (*AnthropicMessagesProvider, error) {
	apiKey := os.Getenv("ANTHROPIC_AUTH_TOKEN")
	if apiKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_AUTH_TOKEN not set")
	}
	baseURL := os.Getenv("ANTHROPIC_BASE_URL")
	if baseURL == "" {
		baseURL = "https://athenai.mihoyo.com"
	}
	return &AnthropicMessagesProvider{
		baseURL: baseURL,
		apiKey:  apiKey,
	}, nil
}

// Stream implements core.Provider.Stream (SKELETON).
func (p *AnthropicMessagesProvider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	return nil, fmt.Errorf("anthropic-messages: Stream not implemented (skeleton)")
}

// Complete implements core.Provider.Complete (SKELETON).
func (p *AnthropicMessagesProvider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	return core.CompleteResponse{}, fmt.Errorf("anthropic-messages: Complete not implemented (skeleton)")
}