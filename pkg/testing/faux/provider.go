// Package faux provides a test Provider with pre-queued responses.
// Zero mock layers above the Provider — tests exercise the real Loop/Hook/Session stack.
package faux

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/akzj/tau/core"
)

// Provider implements core.Provider with pre-queued response sequences.
// Thread-safe. Use QueueStream/QueueComplete before calling Stream/Complete.
type Provider struct {
	mu          sync.Mutex
	streams     []StreamResponse
	streamIdx   int
	completes   []CompleteEntry
	completeIdx int
}

// StreamResponse is a pre-queued streaming response.
type StreamResponse struct {
	Events []core.ProviderEvent // events to replay in order
	Delay  time.Duration        // per-event delay to simulate streaming (0 = instant)
}

// CompleteEntry is a pre-queued complete response.
type CompleteEntry struct {
	Response core.CompleteResponse
	Err      error
}

// New creates a fresh Faux Provider with empty queues.
func New() *Provider {
	return &Provider{}
}

// QueueStream appends streaming responses. Call before starting the Loop.
func (p *Provider) QueueStream(responses ...StreamResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.streams = append(p.streams, responses...)
}

// QueueComplete appends complete responses.
func (p *Provider) QueueComplete(entries ...CompleteEntry) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completes = append(p.completes, entries...)
}

// Stream dequeues and replays the next queued streaming response.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	p.mu.Lock()
	if p.streamIdx >= len(p.streams) {
		p.mu.Unlock()
		return nil, fmt.Errorf("faux: no queued streaming responses (have %d, used %d)", len(p.streams), p.streamIdx)
	}
	resp := p.streams[p.streamIdx]
	p.streamIdx++
	p.mu.Unlock()

	ch := make(chan core.ProviderEvent, len(resp.Events)+1)
	go func() {
		defer close(ch)
		for _, ev := range resp.Events {
			select {
			case <-ctx.Done():
				return // abort propagation
			case ch <- ev:
			}
			if resp.Delay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(resp.Delay):
				}
			}
		}
	}()
	return ch, nil
}

// Complete dequeues and returns the next queued complete response.
func (p *Provider) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	p.mu.Lock()
	if p.completeIdx >= len(p.completes) {
		p.mu.Unlock()
		return core.CompleteResponse{}, fmt.Errorf("faux: no queued complete responses")
	}
	entry := p.completes[p.completeIdx]
	p.completeIdx++
	p.mu.Unlock()
	return entry.Response, entry.Err
}