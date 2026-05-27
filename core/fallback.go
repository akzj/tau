package core

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// FallbackPolicy determines how provider failover is handled.
type FallbackPolicy string

const (
	FallbackSequential FallbackPolicy = "sequential"
	FallbackRoundRobin FallbackPolicy = "round-robin"
	FallbackWeighted   FallbackPolicy = "weighted"
)

// FallbackRouter handles provider failover and load balancing across
// multiple Providers. It implements the Provider interface so it can
// be dropped in anywhere a single Provider is expected — including
// nesting (router-of-routers).
//
// Zero modifications to core/loop.go — injected at session creation.
type FallbackRouter struct {
	mu        sync.RWMutex
	providers []*ProviderEntry
	cursor    atomic.Int64
	policy    FallbackPolicy
}

// ProviderEntry holds a provider with health and weight metadata.
type ProviderEntry struct {
	Provider  Provider
	Name      string
	Weight    int
	healthy   *atomic.Bool // pointer avoids noCopy — safe to pass by value
	lastCheck time.Time
}

// Healthy returns whether the provider is currently healthy.
func (e *ProviderEntry) Healthy() bool { return e.healthy.Load() }

// SetHealthy sets the provider health status.
func (e *ProviderEntry) SetHealthy(v bool) { e.healthy.Store(v) }

// NewFallbackRouter creates a FallbackRouter from pre-built provider entries.
// All providers start healthy.
func NewFallbackRouter(entries []ProviderEntry, policy FallbackPolicy) *FallbackRouter {
	r := &FallbackRouter{
		providers: make([]*ProviderEntry, len(entries)),
		policy:    policy,
	}
	for i := range entries {
		r.providers[i] = &ProviderEntry{
			Provider: entries[i].Provider,
			Name:     entries[i].Name,
			Weight:   entries[i].Weight,
			healthy:  new(atomic.Bool),
		}
		r.providers[i].healthy.Store(true)
	}
	return r
}

// ParseFallbackChain parses a comma-separated provider chain string.
// Example: "openai, google, anthropic" → ["openai", "google", "anthropic"]
func ParseFallbackChain(chain string) []string {
	if chain == "" {
		return nil
	}
	var names []string
	for _, name := range strings.Split(chain, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// ParseProviderWeights parses "name=weight,name=weight" format.
// Example: "anthropic=5,openai=3" → {"anthropic": 5, "openai": 3}
func ParseProviderWeights(spec string) map[string]int {
	weights := make(map[string]int)
	if spec == "" {
		return weights
	}
	for _, pair := range strings.Split(spec, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			name := strings.TrimSpace(parts[0])
			var w int
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%d", &w)
			if w > 0 {
				weights[name] = w
			}
		}
	}
	return weights
}

// ---- core.Provider interface ----

// Stream tries providers according to the policy until one succeeds.
// Unhealthy providers are skipped; on failure the provider is marked
// unhealthy and the next candidate is tried.
func (r *FallbackRouter) Stream(ctx context.Context, req StreamRequest) (<-chan ProviderEvent, error) {
	providers := r.selectProviders()
	var lastErr error
	tried := 0
	for _, entry := range providers {
		if !entry.healthy.Load() {
			continue
		}
		tried++
		ch, err := entry.Provider.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
		entry.healthy.Store(false)
		fmt.Fprintf(os.Stderr, "[fallback] %s failed: %v — trying next\n", entry.Name, err)
		Logger().Warn("fallback: provider failed", "provider", entry.Name, "err", err)
	}
	if tried == 0 {
		return nil, fmt.Errorf("fallback: no healthy providers available (%d total)", len(providers))
	}
	return nil, fmt.Errorf("fallback: all providers failed, last error: %w", lastErr)
}

// Complete tries providers until one succeeds.
func (r *FallbackRouter) Complete(ctx context.Context, req CompleteRequest) (CompleteResponse, error) {
	providers := r.selectProviders()
	var lastErr error
	tried := 0
	for _, entry := range providers {
		if !entry.healthy.Load() {
			continue
		}
		tried++
		resp, err := entry.Provider.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		entry.healthy.Store(false)
		fmt.Fprintf(os.Stderr, "[fallback] %s failed: %v — trying next\n", entry.Name, err)
		Logger().Warn("fallback: provider failed", "provider", entry.Name, "err", err)
	}
	if tried == 0 {
		return CompleteResponse{}, fmt.Errorf("fallback: no healthy providers available (%d total)", len(providers))
	}
	return CompleteResponse{}, fmt.Errorf("fallback: all providers failed, last error: %w", lastErr)
}

// ---- Health checking ----

// StartHealthCheck begins periodic health probing of all providers.
// A nil or zero interval defaults to 60 seconds.
func (r *FallbackRouter) StartHealthCheck(interval time.Duration) {
	if interval <= 0 {
		interval = 60 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			r.runHealthCheck()
		}
	}()
}

func (r *FallbackRouter) runHealthCheck() {
	r.mu.RLock()
	entries := make([]*ProviderEntry, len(r.providers))
	copy(entries, r.providers)
	r.mu.RUnlock()

	for _, entry := range entries {
		// Simple probe: try a minimal Complete call
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := entry.Provider.Complete(ctx, CompleteRequest{
			Model:    ModelSpec{Name: "ping"},
			Messages: []Message{{Role: RoleUser, Content: "ping"}},
		})
		cancel()

		previouslyHealthy := entry.healthy.Load()
		nowHealthy := err == nil
		entry.healthy.Store(nowHealthy)

		if previouslyHealthy && !nowHealthy {
			fmt.Fprintf(os.Stderr, "[fallback] %s became UNHEALTHY\n", entry.Name)
			Logger().Warn("fallback: provider unhealthy", "provider", entry.Name)
		} else if !previouslyHealthy && nowHealthy {
			fmt.Fprintf(os.Stderr, "[fallback] %s RECOVERED\n", entry.Name)
			Logger().Info("fallback: provider recovered", "provider", entry.Name)
		}
		entry.lastCheck = time.Now()
	}
}

// ---- Provider selection strategies ----

func (r *FallbackRouter) selectProviders() []*ProviderEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	switch r.policy {
	case FallbackWeighted:
		return r.weightedSelect()
	case FallbackRoundRobin:
		return r.roundRobinSelect()
	default:
		// sequential: return a copy of the slice as-is
		result := make([]*ProviderEntry, len(r.providers))
		copy(result, r.providers)
		return result
	}
}

func (r *FallbackRouter) weightedSelect() []*ProviderEntry {
	// Build weighted pool of healthy providers
	var pool []*ProviderEntry
	for _, p := range r.providers {
		if !p.healthy.Load() {
			continue
		}
		w := p.Weight
		if w <= 0 {
			w = 1
		}
		for i := 0; i < w; i++ {
			pool = append(pool, p)
		}
	}
	if len(pool) == 0 {
		// All unhealthy: return original order so caller can try them
		result := make([]*ProviderEntry, len(r.providers))
		copy(result, r.providers)
		return result
	}
	// Shuffle for probabilistic distribution
	rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
	return pool
}

func (r *FallbackRouter) roundRobinSelect() []*ProviderEntry {
	// Separate healthy from unhealthy
	healthy := make([]*ProviderEntry, 0, len(r.providers))
	unhealthy := make([]*ProviderEntry, 0)
	for _, p := range r.providers {
		if p.healthy.Load() {
			healthy = append(healthy, p)
		} else {
			unhealthy = append(unhealthy, p)
		}
	}
	if len(healthy) == 0 {
		result := make([]*ProviderEntry, len(r.providers))
		copy(result, r.providers)
		return result
	}
	// Rotate: advance cursor, pick next healthy provider as first
	idx := int(r.cursor.Add(1)-1) % len(healthy)
	if idx < 0 {
		idx = 0
	}
	// Reorder: start from idx, then wrap around, then append unhealthy
	result := make([]*ProviderEntry, 0, len(r.providers))
	result = append(result, healthy[idx:]...)
	result = append(result, healthy[:idx]...)
	result = append(result, unhealthy...)
	return result
}