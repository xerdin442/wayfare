package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/driver/internal/infra/events"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/driver/internal/secrets"
	"github.com/xerdin442/wayfare/services/driver/internal/server"
	"github.com/xerdin442/wayfare/services/driver/internal/service"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/tracing"
	"golang.org/x/sync/errgroup"
)

func main() {
	// Load environment variables
	env := secrets.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Initialize tracer
	tracerCfg := &tracing.TraceConfig{
		ServiceName:       "driver-service",
		Environment:       env.Environment,
		CollectorEndpoint: env.TraceCollectorEndpoint,
		Insecure:          env.Environment == "production",
	}
	shutdown, err := tracing.InitTracer(ctx, tracerCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	defer shutdown(ctx)

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	database := storage.InitDatabase(ctx, env.MongoUri)
	cache := storage.InitCache(ctx, env.RedisUri)

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	// Setup repository and service
	repo := repo.NewDriverRepository(database)
	svc := service.NewDriverService(repo)
	h := events.NewDriverEventsHandler(rmq, cache)

	w := messaging.NewEventWorker(rmq, messaging.AssignDriverQueue)
	w.RegisterHandler(
		h.FindAndAssignDriver,
		messaging.TripEventCreated,
		messaging.TripEventDriverNotAvailable,
		messaging.TripEventDriverNotInterested,
	)

	g.Go(func() error {
		log.Info().Msg("Starting event worker...")
		return w.Start()
	})

	g.Go(func() error {
		log.Info().Msg("Starting server...")

		srv := server.New()
		return srv.Start(svc, env.ServicePort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Driver service stopped")
	}
}
