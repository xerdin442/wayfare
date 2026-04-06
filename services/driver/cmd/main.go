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
	"golang.org/x/sync/errgroup"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Load environment variables
	env := secrets.Load()

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	database := storage.InitializeDatabase(ctx, env.MongoUri)
	cache := storage.InitializeCache(ctx, env.RedisUri)

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	repo := repo.NewDriverRepository(database)
	svc := service.NewDriverService(repo, rmq)
	h := events.NewDriverEventsHandler(repo, rmq, cache)

	w := messaging.NewEventWorker(rmq, messaging.AssignDriverQueue)
	w.RegisterHandler(
		h.FindAndAssignDriver,
		messaging.TripEventCreated,
		messaging.TripEventDriverNotAvailable,
		messaging.TripEventDriverNotInterested,
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
		log.Fatal().Err(err).Msg("Driver service stopped")
	}
}
