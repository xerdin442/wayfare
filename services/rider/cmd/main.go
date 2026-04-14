package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/rider/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/rider/internal/secrets"
	"github.com/xerdin442/wayfare/services/rider/internal/server"
	"github.com/xerdin442/wayfare/services/rider/internal/service"
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

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	repo := repo.NewRiderRepository(database)
	svc := service.NewRiderService(repo, rmq, env.JwtSecret)

	g.Go(func() error {
		log.Info().Msg("Starting server...")

		srv := server.New()
		return srv.Start(svc, env.ServicePort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Rider service stopped")
	}
}
