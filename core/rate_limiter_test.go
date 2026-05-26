package core

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterBasic(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	for i := 0; i < 10; i++ {
		if !rl.Allow() {
			t.Errorf("expected token %d available", i)
		}
	}
	if rl.Allow() {
		t.Error("expected no token after burst exhausted")
	}
}

func TestRateLimiterRefill(t *testing.T) {
	rl := NewRateLimiter(100, 2)
	rl.Allow()
	rl.Allow()
	time.Sleep(30 * time.Millisecond)
	if !rl.Allow() {
		t.Error("expected token after refill")
	}
}

func TestRateLimiterContextCancel(t *testing.T) {
	rl := NewRateLimiter(0, 0) // rate=10, burst=3 defaults
	// Consume all burst tokens
	for i := 0; i < 3; i++ {
		rl.Allow()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := rl.Wait(ctx)
	if err == nil {
		t.Error("expected context deadline exceeded error")
	}
}

func TestRateLimiterRegistry(t *testing.T) {
	reg := NewRateLimiterRegistry()
	rl1 := reg.Get("openai")
	rl2 := reg.Get("openai")
	if rl1 != rl2 {
		t.Error("expected same limiter for same provider")
	}
	rl3 := reg.Get("anthropic")
	if rl1 == rl3 {
		t.Error("expected different limiter for different provider")
	}
}
