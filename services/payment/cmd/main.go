package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/payment/internal/infra/events"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/payment/internal/secrets"
	"github.com/xerdin442/wayfare/services/payment/internal/server"
	"github.com/xerdin442/wayfare/services/payment/internal/service"
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

	// Initialize database
	database := storage.InitializeDatabase(ctx, env.MongoUri)

	// Initialize cache
	cache := storage.InitializeCache(ctx, env.RedisUri)

	// Initialize message bus
	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	// Setup repository and service
	repo := repo.NewPaymentRepository(database)
	svc := service.NewPaymentService(repo, cache)
	h := events.NewPaymentEventsHandler(repo, rmq, cache)

	w := messaging.NewEventWorker(rmq, messaging.PaymentQueue)
	w.RegisterHandler(h.HandleWebhook, messaging.PaymentEventWebhookReceived)

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
		log.Fatal().Err(err).Msg("Payment service stopped")
	}
}
