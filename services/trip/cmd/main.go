package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/trip/internal/server"
	"github.com/xerdin442/wayfare/shared/secrets"
)

func main() {
	// Load environment variables
	env := secrets.Load()

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Start server
	srv := server.New(env)
	if err := srv.Start(); err != nil {
		log.Fatal().Err(err).Msg("Server initialization failed")
	}
}
