package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/driver/internal/service"
	db "github.com/xerdin442/wayfare/shared/database"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/secrets"
	"google.golang.org/grpc"
)

type Server struct {
	grpcServer *grpc.Server
	env        *secrets.Secrets
}

func New(s *secrets.Secrets) *Server {
	return &Server{
		env: s,
	}
}

func (s *Server) Start() error {
	// Create listener
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", s.env.ServicePort))
	if err != nil {
		return fmt.Errorf("Failed to listen: %w", err)
	}

	// Create gRPC server
	s.grpcServer = grpc.NewServer()

	// Initialize database
	database := db.InitializeDatabase(context.Background(), s.env.MongoUri)

	// Initialize repository
	tripRepo := repo.NewDriverRepository(database)

	// Register service
	rpc.RegisterDriverServiceServer(s.grpcServer, service.NewDriverService(tripRepo))

	log.Info().Int("port", s.env.ServicePort).Msg("Starting gRPC server...")

	// Start server
	errChan := make(chan error, 1)
	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			errChan <- err
		}
	}()

	// Wait for termination signal or error
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return fmt.Errorf("Server error: %w", err)
	case sig := <-sigChan:
		log.Info().Str("signal", sig.String()).Msg("Shutting down server...")
		s.Shutdown()
		return nil
	}
}

func (s *Server) Shutdown() {
	log.Info().Msg("Graceful shutdown initiated")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Graceful stop
	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-ctx.Done():
		log.Warn().Msg("Shutdown timeout, forcing stop...")
		s.grpcServer.Stop()
	case <-stopped:
		log.Info().Msg("Server stopped gracefully")
	}
}
