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

	// Initialize analytics
	conn, err := analytics.SetupProvider(ctx, &analytics.AnalyticsConfig{
		ConnectionUri: env.ClickHouseUri,
		Username:      env.ClickHouseUsername,
		Password:      env.ClickHousePassword,
	})
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize analytics")
	}
	defer conn.Close()

	// Initialize logger
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Turn off debug messages in production
	if env.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	cache := storage.InitCache(ctx, env.RedisUri)

	rmq := messaging.NewRabbitMQ(env.AmqpUri)
	defer rmq.Close()

	grpcClients := client.NewRegistry(env.ServicePort, env.Environment == "development")
	defer grpcClients.Close()

	baseCfg := &base.Config{
		Env:     env,
		Cache:   cache,
		Clients: grpcClients,
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

	gatewayHandler := events.NewGatewayEventsHandler(baseCfg)
	w1 := messaging.NewEventWorker(rmq, messaging.GatewayQueue)
	w1.RegisterHandler(gatewayHandler.HandleOutgoingWebsocketMessages, "user.*")

	analyticsHandler := analytics.NewAnalyticsEventHandler(conn)
	w2 := messaging.NewEventWorker(rmq, messaging.AnalyticsQueue)
	w2.RegisterHandler(analyticsHandler.HandleAnalyticsEvent, messaging.AnalyticsEventUpdate)

	app := &application{
		port:   env.GatewayPort,
		config: baseCfg,
	}

	g.Go(func() error {
		log.Info().Msg("Starting gateway event worker...")
		return w1.Start()
	})

	g.Go(func() error {
		log.Info().Msg("Starting analytics event worker...")
		return w2.Start()
	})

	g.Go(func() error {
		log.Info().Msg("Starting API gateway...")
		return app.serve()
	})

	if err := g.Wait(); err != nil {
		log.Fatal().Err(err).Msg("API Gateway stopped")
	}
}
