// Package mock provides a deterministic mock Provider for evaluation.
package mock

import (
	"context"
	"fmt"
	"time"

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider with configurable fixed responses.
type Provider struct {
	Response string
	Tools    []string // tool names to emit as ProvToolCallStart
	Err      error
}

// Stream implements core.Provider.Stream.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	if p.Err != nil {
		return nil, p.Err
	}
	ch := make(chan core.ProviderEvent, len(p.Tools)+2)
	go func() {
		defer close(ch)
		msgID := fmt.Sprintf("mock-%d", time.Now().UnixNano())
		ch <- core.ProviderEvent{Type: core.ProvMessageStart, MessageID: msgID}
		for _, tool := range p.Tools {
			ch <- core.ProviderEvent{Type: core.ProvToolCallStart, MessageID: msgID, ToolCallID: "mock-" + tool, ToolName: tool}
			ch <- core.ProviderEvent{Type: core.ProvToolCallEnd, MessageID: msgID, ToolCallID: "mock-" + tool}
		}
		if p.Response != "" {
			ch <- core.ProviderEvent{Type: core.ProvContentDelta, MessageID: msgID, ContentDelta: p.Response}
		}
		ch <- core.ProviderEvent{Type: core.ProvMessageEnd, MessageID: msgID}
	}()
	return ch, nil
}

// Complete implements core.Provider.Complete.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	return core.CompleteResponse{Content: p.Response}, p.Err
}
