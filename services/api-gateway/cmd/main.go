package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/client"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/events"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/secrets"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/storage"
	"golang.org/x/sync/errgroup"
)

type application struct {
	port   int
	config *base.Config
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Load environment variables
	env := secrets.Load()

	// Turn off debug messages in production
	if env.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize Redis cache
	cache := storage.InitializeCache(ctx, env.RedisUri)

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	// Initialize gRPC clients
	clients := client.NewRegistry()
	defer clients.Close()

	baseCfg := &base.Config{
		Env:     env,
		Cache:   cache,
		Clients: clients,
		Queue:   rmq,
	}

	h := events.NewGatewayEventsHandler(baseCfg)
	w := messaging.NewEventWorker(rmq, messaging.GatewayQueue)
	w.RegisterHandler(h.HandleGatewayQueueEvents, "user.*")

	app := &application{
		port:   env.GatewayPort,
		config: baseCfg,
	}

	g.Go(func() error {
		log.Info().Msg("Starting event worker...")
		return w.Start(ctx)
	})

	g.Go(func() error {
		log.Info().Msg("Starting API gateway...")
		return app.serve()
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("API Gateway stopped")
	}
}
