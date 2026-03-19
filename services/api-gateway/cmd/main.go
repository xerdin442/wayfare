package main

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/secrets"
)

type application struct {
	port int
	contracts.Base
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

	// Improve readability of logs in development
	if env.Environment == "development" {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})
	}

	// Initialize cache
	cacheOpts, err := redis.ParseURL(env.RedisUri)
	if err != nil {
		log.Fatal().Msg("Invalid Redis connection URL")
	}

	cache := redis.NewClient(cacheOpts)

	app := &application{
		port: env.Port,
		Base: contracts.Base{
			Env:   env,
			Cache: cache,
		},
	}

	// Start the API Gateway
	if err := app.serve(); err != nil {
		log.Fatal().Err(err).Msg("Failed to start API Gateway")
	}
}
