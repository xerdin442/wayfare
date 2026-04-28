package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

func InitCache(ctx context.Context, uri string) *redis.Client {
	// Parse connection URI
	cacheOpts, err := redis.ParseURL(uri)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid Redis connection URI")
	}

	// Initialize cache
	cache := redis.NewClient(cacheOpts)

	// Instrument cache
	if err := redisotel.InstrumentTracing(cache); err != nil {
		log.Fatal().Err(err).Msg("Failed to instrument Redis for tracing")
	}
	if err := redisotel.InstrumentMetrics(cache); err != nil {
		log.Fatal().Err(err).Msg("Failed to instrument Redis for metrics")
	}

	// Ping Redis to ensure connection is alive
	var pingErr error
	for range 3 {
		pingErr = cache.Ping(context.Background()).Err()
		if pingErr == nil {
			break
		}

		log.Warn().Msg("Waiting for Redis...")
		time.Sleep(time.Second * 2)
	}

	if pingErr != nil {
		log.Fatal().Err(pingErr).Msg("Could not connect to Redis after 3 attempts. Exiting...")
	}

	return cache
}
