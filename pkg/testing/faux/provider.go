// Package faux provides a test Provider with pre-queued responses.
// Zero mock layers above the Provider — tests exercise the real Loop/Hook/Session stack.
package faux

import (
	"context"
	"fmt"
	"math/rand"
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
	sessions    map[string]*cacheState                // session-keyed prompt cache tracking
	factory     func(context.Context, core.StreamRequest) []StreamResponse // dynamic response factory (overrides queue)
}

type cacheState struct {
	commonPrefix int   // tracks repeated prefix tokens
	cacheReads   []int // per-turn cache read counts
	cacheWrites  []int // per-turn cache write counts
}

// StreamResponse is a pre-queued streaming response.
type StreamResponse struct {
	Events          []core.ProviderEvent // events to replay in order
	Delay           time.Duration        // per-event delay to simulate streaming (0 = instant)
	TokensPerSecond int                  // realistic delta streaming: 0 = instant (use Delay), >0 = split ContentDelta into chunks
	MinTokenSize    int                  // minimum chars per delta chunk (default 3)
	MaxTokenSize    int                  // maximum chars per delta chunk (default 10)
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

// SetFactory sets a dynamic response factory (overrides queue).
func (p *Provider) SetFactory(fn func(context.Context, core.StreamRequest) []StreamResponse) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.factory = fn
}

// CacheStats returns cache reads/writes for a session key.
func (p *Provider) CacheStats(sessionKey string) (reads, writes []int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cs, ok := p.sessions[sessionKey]; ok {
		return cs.cacheReads, cs.cacheWrites
	}
	return nil, nil
}

// Stream dequeues and replays the next queued streaming response.
func (p *Provider) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	p.mu.Lock()

	// Check factory first
	if p.factory != nil {
		responses := p.factory(ctx, req)
		p.mu.Unlock()
		if len(responses) == 0 {
			return nil, fmt.Errorf("faux: factory returned no responses")
		}
		resp := responses[0]
		ch := make(chan core.ProviderEvent, len(resp.Events)+1)
		go p.replayEvents(ctx, ch, resp)
		return ch, nil
	}

	// Queue-based path
	if p.streamIdx >= len(p.streams) {
		p.mu.Unlock()
		return nil, fmt.Errorf("faux: no queued streaming responses (have %d, used %d)", len(p.streams), p.streamIdx)
	}
	resp := p.streams[p.streamIdx]
	p.streamIdx++

	// Simulate prompt cache
	if p.sessions == nil {
		p.sessions = make(map[string]*cacheState)
	}
	sessID := fmt.Sprintf("%v", req.Model.Name) // simplified session key
	cs := p.sessions[sessID]
	if cs == nil {
		cs = &cacheState{}
		p.sessions[sessID] = cs
	}
	cs.cacheReads = append(cs.cacheReads, cs.commonPrefix)
	newPrefix := len(req.Messages) // simplified: message count as common prefix
	cs.cacheWrites = append(cs.cacheWrites, newPrefix-cs.commonPrefix)
	cs.commonPrefix = newPrefix
	p.mu.Unlock()

	ch := make(chan core.ProviderEvent, len(resp.Events)+1)
	go p.replayEvents(ctx, ch, resp)
	return ch, nil
}

// replayEvents replays a StreamResponse onto a channel with delta splitting and abort propagation.
func (p *Provider) replayEvents(ctx context.Context, ch chan<- core.ProviderEvent, resp StreamResponse) {
	defer close(ch)
	for _, ev := range resp.Events {
		// Delta streaming: split ContentDelta into realistic chunks
		if ev.Type == core.ProvContentDelta && resp.TokensPerSecond > 0 && ev.ContentDelta != "" {
			chunks := splitIntoChunks(ev.ContentDelta, resp.MinTokenSize, resp.MaxTokenSize)
			delayPerChunk := time.Second / time.Duration(resp.TokensPerSecond)
			for _, chunk := range chunks {
				select {
				case <-ctx.Done():
					ch <- core.ProviderEvent{
						Type:         core.ProvContentDelta,
						ContentDelta: "\n[aborted]",
					}
					return
				case ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					MessageID:    ev.MessageID,
					ContentDelta: chunk,
				}:
				}
				if delayPerChunk > 0 {
					select {
					case <-ctx.Done():
						ch <- core.ProviderEvent{
							Type:         core.ProvContentDelta,
							ContentDelta: "\n[aborted]",
						}
						return
					case <-time.After(delayPerChunk):
					}
				}
			}
		} else {
			select {
			case <-ctx.Done():
				ch <- core.ProviderEvent{
					Type:         core.ProvContentDelta,
					ContentDelta: "\n[aborted]",
				}
				return
			case ch <- ev:
			}
			if resp.Delay > 0 {
				select {
				case <-ctx.Done():
					ch <- core.ProviderEvent{
						Type:         core.ProvContentDelta,
						ContentDelta: "\n[aborted]",
					}
					return
				case <-time.After(resp.Delay):
				}
			}
		}
	}
}

// splitIntoChunks splits text into realistic token-sized chunks.
func splitIntoChunks(text string, minSize, maxSize int) []string {
	if minSize <= 0 {
		minSize = 3
	}
	if maxSize <= 0 {
		maxSize = 10
	}
	if len(text) <= maxSize {
		return []string{text}
	}

	var chunks []string
	for len(text) > 0 {
		size := minSize + rand.Intn(maxSize-minSize+1)
		if size > len(text) {
			size = len(text)
		}
		chunks = append(chunks, text[:size])
		text = text[size:]
	}
	return chunks
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