package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWithBackoffSucceedsAfterRetries(t *testing.T) {
	cfg := &RetryConfig{MaxRetries: 3, InitialWait: 1 * time.Millisecond, MaxWait: 4 * time.Millisecond}
	var attempts int

	err := WithBackoff(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary failure")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected retry to succeed, got %v", err)
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestWithBackoffReturnsContextError(t *testing.T) {
	cfg := &RetryConfig{MaxRetries: 3, InitialWait: 1 * time.Millisecond, MaxWait: 4 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WithBackoff(ctx, cfg, func() error {
		return errors.New("boom")
	})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
