package core

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds retry parameters.
type RetryConfig struct {
	MaxRetries   int           // max retry attempts (0 = no retry)
	InitialDelay time.Duration // base delay before first retry
	MaxDelay     time.Duration // cap on exponential backoff
	Multiplier   float64       // backoff multiplier (default 2.0)
	Jitter       float64       // random jitter factor (0.0-1.0, default 0.1)
}

// DefaultRetryConfig returns sensible retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// Retry executes fn with retry logic. Retries only on transient errors.
func Retry[T any](ctx context.Context, cfg RetryConfig, fn func(context.Context) (T, error)) (T, error) {
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 3
	}
	if cfg.InitialDelay <= 0 {
		cfg.InitialDelay = 1 * time.Second
	}
	if cfg.MaxDelay <= 0 {
		cfg.MaxDelay = 30 * time.Second
	}
	if cfg.Multiplier <= 0 {
		cfg.Multiplier = 2.0
	}
	if cfg.Jitter <= 0 {
		cfg.Jitter = 0.1
	}

	var zero T
	var lastErr error

	for attempt := 0; attempt <= cfg.MaxRetries; attempt++ {
		result, err := fn(ctx)
		if err == nil {
			return result, nil
		}

		lastErr = err

		if !isTransient(err) {
			return zero, err
		}

		if attempt < cfg.MaxRetries {
			delay := time.Duration(float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt)))
			if delay > cfg.MaxDelay {
				delay = cfg.MaxDelay
			}

			if cfg.Jitter > 0 {
				jitterMs := time.Duration(float64(delay) * cfg.Jitter * (rand.Float64()*2 - 1))
				delay += jitterMs
			}
			if delay < 0 {
				delay = cfg.InitialDelay
			}

			Logger().Warn("retry: attempt", "attempt", attempt+1, "maxRetries", cfg.MaxRetries, "delay", delay,
				"err", err.Error()[:min(len(err.Error()), 100)])

			select {
			case <-ctx.Done():
				return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
			case <-time.After(delay):
			}
		}
	}

	return zero, fmt.Errorf("retry exhausted after %d attempts: %w", cfg.MaxRetries+1, lastErr)
}

// isTransient returns true if the error is retryable.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return containsAny(s, "429", "503", "timeout", "connection reset", "EOF", "broken pipe", "connection refused")
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
