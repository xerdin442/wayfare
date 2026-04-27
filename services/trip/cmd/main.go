package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/trip/internal/infra/events"
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/trip/internal/secrets"
	"github.com/xerdin442/wayfare/services/trip/internal/server"
	"github.com/xerdin442/wayfare/services/trip/internal/service"
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

	// Initialize tracing
	tracerCfg := &tracing.TraceConfig{
		ServiceName:       "trip-service",
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

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	repo := repo.NewTripRepository(database)
	svc := service.NewTripService(repo, rmq)
	h := events.NewTripEventsHandler(repo, rmq)

	w := messaging.NewEventWorker(rmq, messaging.TripUpdateQueue)
	w.RegisterHandler(
		h.HandleTripUpdate,
		messaging.TripEventDriverAssigned,
		messaging.TripEventNoDriversFound,
		messaging.DriverCmdTripPickup,
		messaging.TripCmdCancelled,
		messaging.TripCmdCompleted,
		messaging.TripCmdAborted,
	)

	g.Go(func() error {
		log.Info().Msg("Starting event worker...")
		return w.Start(ctx)
	})

	g.Go(func() error {
		log.Info().Msg("Starting server...")

		srv := server.New()
		return srv.Start(svc, env.ServicePort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Trip service stopped")
	}
}
