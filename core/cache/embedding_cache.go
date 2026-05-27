package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// EmbeddingCacheMiddleware wraps a core.Embedder with vector caching.
// Embedded results are stored as JSON BLOBs keyed by embedder name + text.
type EmbeddingCacheMiddleware struct {
	inner   core.Embedder
	cache   CacheBackend
	ttl     time.Duration
	enabled bool
}

// NewEmbeddingCache wraps inner with a 24-hour TTL by default.
func NewEmbeddingCache(inner core.Embedder, cache CacheBackend) *EmbeddingCacheMiddleware {
	return &EmbeddingCacheMiddleware{
		inner:   inner,
		cache:   cache,
		ttl:     24 * time.Hour,
		enabled: true,
	}
}

// SetEnabled toggles caching.
func (c *EmbeddingCacheMiddleware) SetEnabled(v bool) { c.enabled = v }

// Embed satisfies core.Embedder. On a cache hit the embedding is deserialized
// from JSON; on a miss the inner embedder is called and the result is cached.
func (c *EmbeddingCacheMiddleware) Embed(ctx context.Context, text string) (core.Embedding, error) {
	key := EmbeddingCacheKey(c.inner.Name(), text)

	if cached, ok := c.cache.Get(key); ok && c.enabled {
		var emb core.Embedding
		if err := json.Unmarshal(cached, &emb); err == nil {
			fmt.Fprintf(os.Stderr, "[cache] embedding hit\n")
			return emb, nil
		}
	}

	emb, err := c.inner.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	if embJSON, err := json.Marshal(emb); err == nil {
		c.cache.Set(key, embJSON, c.ttl)
	}
	return emb, nil
}

// Name returns the inner embedder's name.
func (c *EmbeddingCacheMiddleware) Name() string { return c.inner.Name() }

// Dimensions returns the inner embedder's dimensionality.
func (c *EmbeddingCacheMiddleware) Dimensions() int { return c.inner.Dimensions() }
