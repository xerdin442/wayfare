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

	// Setup repository and service
	repo := repo.NewRiderRepository(database)
	svc := service.NewRiderService(repo)

	g.Go(func() error {
		log.Info().Msg("Starting server...")

		srv := server.New()
		return srv.Start(svc, env.ServicePort)
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("Rider service stopped")
	}
}
