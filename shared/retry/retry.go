package retry

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
)

type RetryConfig struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
}

// DefaultConfig returns a RetryConfig with default values
func DefaultConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     10 * time.Second,
	}
}

// WithBackoff executes the given operation with exponential backoff retry logic
func WithBackoff(ctx context.Context, cfg *RetryConfig, operation func() error) error {
	var err error
	wait := cfg.InitialWait

	for attempt := 1; attempt <= cfg.MaxRetries; attempt++ {
		if attempt > 1 {
			log.Info().Msgf("Retry attempt %d/%d after %v", attempt, cfg.MaxRetries, wait)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}

			// Exponential backoff with max wait cap
			wait *= 2
			if wait > cfg.MaxWait {
				wait = cfg.MaxWait
			}
		}

		if err = operation(); err == nil {
			return nil
		}
	}

	log.Error().Err(err).Msgf("Operation failed after %d attempts", cfg.MaxRetries)
	return err
}
