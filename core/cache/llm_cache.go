package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/akzj/tau/core"
)

// LLMCacheMiddleware wraps a core.Provider with response caching.
// Cache hits return the previously stored text as a single event stream;
// cache misses forward to the inner provider and cache the accumulated text.
type LLMCacheMiddleware struct {
	inner   core.Provider
	cache   CacheBackend
	ttl     time.Duration
	enabled bool
}

// NewLLMCache wraps inner. ttl defaults to 1 hour when ≤ 0.
func NewLLMCache(inner core.Provider, cache CacheBackend, ttl time.Duration) *LLMCacheMiddleware {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &LLMCacheMiddleware{inner: inner, cache: cache, ttl: ttl, enabled: true}
}

// SetEnabled toggles caching. When false every request passes through.
func (c *LLMCacheMiddleware) SetEnabled(v bool) { c.enabled = v }

// Stream satisfies core.Provider.
func (c *LLMCacheMiddleware) Stream(ctx context.Context, req core.StreamRequest) (<-chan core.ProviderEvent, error) {
	digest := llmStreamDigest(req)
	key := LLMCacheKey(string(req.Model.API), digest)

	if cached, ok := c.cache.Get(key); ok && c.enabled {
		fmt.Fprintf(os.Stderr, "[cache] llm hit (key: %s)\n", key[:8])
		return cachedStreamEvents(string(cached)), nil
	}

	ch, err := c.inner.Stream(ctx, req)
	if err != nil {
		return nil, err
	}

	out := make(chan core.ProviderEvent, 64)
	go func() {
		defer close(out)
		var buf strings.Builder
		for ev := range ch {
			out <- ev
			if ev.Type == core.ProvContentDelta {
				buf.WriteString(ev.ContentDelta)
			}
		}
		// Cache the full text after the stream completes.
		if buf.Len() > 0 {
			c.cache.Set(key, []byte(buf.String()), c.ttl)
		}
	}()
	return out, nil
}

// Complete satisfies core.Provider.
func (c *LLMCacheMiddleware) Complete(ctx context.Context, req core.CompleteRequest) (core.CompleteResponse, error) {
	digest := llmCompleteDigest(req)
	key := LLMCacheKey(string(req.Model.API), digest)

	if cached, ok := c.cache.Get(key); ok && c.enabled {
		fmt.Fprintf(os.Stderr, "[cache] llm hit\n")
		return core.CompleteResponse{Content: string(cached)}, nil
	}

	resp, err := c.inner.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if resp.Content != "" {
		c.cache.Set(key, []byte(resp.Content), c.ttl)
	}
	return resp, nil
}

// ---- helpers ---------------------------------------------------------------

func llmStreamDigest(req core.StreamRequest) string {
	// Use messages + system-prompt as the distinguishing payload.
	b, _ := json.Marshal(struct {
		Model        string
		SystemPrompt string
		Messages     any
	}{
		Model:        req.Model.Name,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
	})
	return string(b)
}

func llmCompleteDigest(req core.CompleteRequest) string {
	b, _ := json.Marshal(struct {
		Model        string
		SystemPrompt string
		Messages     any
	}{
		Model:        req.Model.Name,
		SystemPrompt: req.SystemPrompt,
		Messages:     req.Messages,
	})
	return string(b)
}

// cachedStreamEvents returns a buffered channel that emits
// a single ProvContentDelta followed by a ProvMessageEnd, then closes.
func cachedStreamEvents(text string) chan core.ProviderEvent {
	ch := make(chan core.ProviderEvent, 2)
	ch <- core.ProviderEvent{
		Type:         core.ProvContentDelta,
		ContentDelta: text,
	}
	ch <- core.ProviderEvent{
		Type: core.ProvMessageEnd,
	}
	close(ch)
	return ch
}
