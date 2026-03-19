package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func (app *application) serve() error {
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", app.port),
		Handler:      app.routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	log.Info().Msgf("Starting API Gateway on port %d...", app.port)
	return server.ListenAndServe()
}
