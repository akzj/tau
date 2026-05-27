package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/akzj/tau/core"
)

// toolTTLs defines conservative per-tool TTL overrides.
// Tools not listed here default to 60 s.
var toolTTLs = map[string]time.Duration{
	"web_search":   5 * time.Minute,
	"browse":       10 * time.Minute,
	"git_log":      30 * time.Second,
	"git_diff":     30 * time.Second,
	"git_branch":   30 * time.Second,
	"read":         10 * time.Second,
	"grep":         10 * time.Second,
	"glob":         30 * time.Second,
	"rag_search":   5 * time.Minute,
	"list":         10 * time.Second,
	"workspace_diag": 5 * time.Minute,
}

// ToolCacheMiddleware wraps a core.Tool with result caching.
// Callers use WrapTool() to decorate an existing Tool; the
// returned tool transparently caches Execute results.
type ToolCacheMiddleware struct {
	cache   CacheBackend
	enabled bool
}

// NewToolCache creates a middleware that can wrap any core.Tool.
func NewToolCache(cache CacheBackend) *ToolCacheMiddleware {
	return &ToolCacheMiddleware{cache: cache, enabled: true}
}

// SetEnabled toggles caching for every tool wrapped by this middleware.
func (m *ToolCacheMiddleware) SetEnabled(v bool) { m.enabled = v }

// WrapTool returns a new core.Tool whose Execute calls are cached.
// The original Tool is not mutated.
func (m *ToolCacheMiddleware) WrapTool(t core.Tool) core.Tool {
	originalExecute := t.Execute
	originalName := t.Name

	t.Execute = func(
		ctx context.Context,
		callID string,
		params any,
		onUpdate func(core.PartialResult),
	) (core.ToolResult, error) {

		if !m.enabled {
			return originalExecute(ctx, callID, params, onUpdate)
		}

		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return originalExecute(ctx, callID, params, onUpdate)
		}

		key := ToolCacheKey(originalName, string(paramsJSON))
		ttl := toolTTL(originalName)

		if cached, ok := m.cache.Get(key); ok {
			var result core.ToolResult
			if json.Unmarshal(cached, &result) == nil {
				fmt.Fprintf(os.Stderr, "[cache] tool hit (%s)\n", originalName)
				return result, nil
			}
		}

		result, err := originalExecute(ctx, callID, params, onUpdate)
		if err != nil {
			return result, err
		}

		if resultJSON, err := json.Marshal(result); err == nil {
			m.cache.Set(key, resultJSON, ttl)
		}
		return result, nil
	}

	return t
}

func toolTTL(name string) time.Duration {
	if t, ok := toolTTLs[name]; ok {
		return t
	}
	return 60 * time.Second
}
