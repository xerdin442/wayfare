package main

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	"github.com/xerdin442/wayfare/shared/secrets"
	"github.com/xerdin442/wayfare/shared/storage"
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

	// Initialize Redis cache
	cache := storage.InitializeCache(context.Background(), env.RedisUri)

	// Initialize gRPC clients
	clients := client.NewRegistry()
	defer clients.Close()

	app := &application{
		port: env.GatewayPort,
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
