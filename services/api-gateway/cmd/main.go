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
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/metrics"
	"github.com/xerdin442/wayfare/shared/storage"
	"github.com/xerdin442/wayfare/shared/tracing"
	"golang.org/x/sync/errgroup"
)

type application struct {
	port   int
	config *base.Config
}

func main() {
	// Load environment variables
	env := secrets.Load()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	// Initialize tracer
	tracerCfg := &tracing.TraceConfig{
		ServiceName:       "api-gateway",
		Environment:       env.Environment,
		CollectorEndpoint: env.TraceCollectorEndpoint,
		Insecure:          env.Environment != "production",
	}
	traceShutdown, err := tracing.InitTracer(ctx, tracerCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize tracer")
	}
	defer traceShutdown(ctx)

	// Initialize metrics
	metricsCfg := &metrics.MetricConfig{
		ServiceName: "api-gateway",
		Environment: env.Environment,
	}
	metricsShutdown, err := metrics.InitMetrics(ctx, metricsCfg)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize metrics")
	}
	defer metricsShutdown(ctx)

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Turn off debug messages in production
	if env.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	cache := storage.InitCache(ctx, env.RedisUri)

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
		Uploader: &storage.FileUploadConfig{
			Folder:      "wayfare/uploads",
			CloudName:   env.CloudinaryName,
			ApiKey:      env.CloudinaryApiKey,
			CloudSecret: env.CloudinarySecret,
		},
		Tracer:     tracing.GetTracer(tracerCfg.ServiceName),
		HttpClient: tracing.NewHttpClient(),
	}

	h := events.NewGatewayEventsHandler(baseCfg)
	w := messaging.NewEventWorker(rmq, messaging.GatewayQueue)
	w.RegisterHandler(h.HandleOutgoingWebsocketMessages, "user.*")

	app := &application{
		port:   env.GatewayPort,
		config: baseCfg,
	}

	g.Go(func() error {
		log.Info().Msg("Starting event worker...")
		return w.Start()
	})

	g.Go(func() error {
		log.Info().Msg("Setting up analytics provider...")

		cfg := &analytics.AnalyticsConfig{
			Bus:           rmq,
			ConnectionUri: env.ClickHouseUri,
			Username:      env.ClickHouseUsername,
			Password:      env.ClickHousePassword,
		}

		return analytics.SetupProvider(ctx, cfg)
	})

	g.Go(func() error {
		log.Info().Msg("Starting API gateway...")
		return app.serve()
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("API Gateway stopped")
	}
}
