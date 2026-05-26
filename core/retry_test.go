package core

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRetrySuccess(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, InitialDelay: 1 * time.Millisecond}
	attempts := 0
	result, err := Retry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		attempts++
		if attempts < 2 {
			return 0, errors.New("503 Service Unavailable")
		}
		return 42, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryMaxExceeded(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 2, InitialDelay: 1 * time.Millisecond}
	_, err := Retry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		return 0, errors.New("503 Service Unavailable")
	})
	if err == nil {
		t.Error("expected error after exhausting retries")
	}
}

func TestRetryContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := RetryConfig{MaxRetries: 5, InitialDelay: 100 * time.Millisecond}

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Retry(ctx, cfg, func(ctx context.Context) (int, error) {
		return 0, errors.New("503 Service Unavailable")
	})
	if err == nil {
		t.Error("expected context cancellation error")
	}
}

func TestRetryNonRetryable(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, InitialDelay: 1 * time.Millisecond}
	attempts := 0
	_, err := Retry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		attempts++
		return 0, errors.New("400 Bad Request")
	})
	if err == nil {
		t.Error("expected error for non-retryable status")
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt (no retry), got %d", attempts)
	}
}

func TestRetryJitter(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 1, InitialDelay: 10 * time.Millisecond, Jitter: 0.5}
	attempts := 0
	_, _ = Retry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		attempts++
		return 0, errors.New("503")
	})
	if attempts != 2 {
		t.Errorf("expected 2 attempts, got %d", attempts)
	}
}

func TestRetryFirstSuccess(t *testing.T) {
	cfg := RetryConfig{MaxRetries: 3, InitialDelay: 1 * time.Millisecond}
	result, err := Retry(context.Background(), cfg, func(ctx context.Context) (int, error) {
		return 99, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != 99 {
		t.Errorf("expected 99, got %d", result)
	}
}
