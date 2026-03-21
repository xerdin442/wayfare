package main

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	"github.com/xerdin442/wayfare/shared/secrets"
)

type application struct {
	port   int
	config base.Config
}

func main() {
	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Load environment variables
	env := secrets.Load()

	// Turn off debug messages in production
	if env.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Parse Redis connection URI
	cacheOpts, err := redis.ParseURL(env.RedisUri)
	if err != nil {
		log.Fatal().Msg("Invalid Redis connection URI")
	}

	// Initialize cache and test connection
	cache := redis.NewClient(cacheOpts)
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

	// Initialize gRPC clients
	clients := client.NewRegistry(env)
	defer clients.Close()

	app := &application{
		port: env.Port,
		config: base.Config{
			Env:     env,
			Cache:   cache,
			Clients: clients,
		},
	}

	// Start the API Gateway
	if err := app.serve(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start API Gateway")
	}
}
