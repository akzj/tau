package core

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	rate       float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a token bucket with the given rate and burst.
func NewRateLimiter(ratePerSec int, burst int) *RateLimiter {
	if ratePerSec <= 0 {
		ratePerSec = 10
	}
	if burst <= 0 {
		burst = 3
	}
	return &RateLimiter{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		rate:       float64(ratePerSec),
		lastRefill: time.Now(),
	}
}

// Allow returns true if a token is immediately available. Non-blocking.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

// Wait blocks until a token is available or ctx is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()
		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		waitTime := time.Duration((1 - rl.tokens) / rl.rate * float64(time.Second))
		rl.mu.Unlock()

		if waitTime < time.Millisecond {
			waitTime = time.Millisecond
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now
}

// RateLimiterRegistry maps provider names to rate limiters.
type RateLimiterRegistry struct {
	mu       sync.Mutex
	limiters map[string]*RateLimiter
}

// NewRateLimiterRegistry creates a rate limiter registry.
func NewRateLimiterRegistry() *RateLimiterRegistry {
	return &RateLimiterRegistry{limiters: make(map[string]*RateLimiter)}
}

// Get returns the rate limiter for a provider, creating one with defaults if needed.
func (r *RateLimiterRegistry) Get(provider string) *RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rl, ok := r.limiters[provider]; ok {
		return rl
	}
	rl := NewRateLimiter(10, 3)
	r.limiters[provider] = rl
	return rl
}

// SetRate sets the rate for a specific provider.
func (r *RateLimiterRegistry) SetRate(provider string, ratePerSec, burst int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limiters[provider] = NewRateLimiter(ratePerSec, burst)
}

// GlobalRateLimiters is the default rate limiter registry.
var GlobalRateLimiters = NewRateLimiterRegistry()

// SetGlobalRateLimit sets rate limits for all known providers.
func SetGlobalRateLimit(ratePerSec int) {
	providers := []string{"openai", "anthropic", "google", "azure", "mistral", "bedrock", "vertex",
		"openai-completions", "anthropic-messages", "google-generative-ai", "azure-openai-responses"}
	for _, provider := range providers {
		GlobalRateLimiters.SetRate(provider, ratePerSec, ratePerSec/3)
	}
}
