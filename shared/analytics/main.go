package analytics

import (
	"context"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xerdin442/wayfare/shared/messaging"
)

type AnalyticsConfig struct {
	Bus           messaging.MessageBus
	ConnectionUri string
	Username      string
	Password      string
}

func SetupAnalyticsProvider(ctx context.Context, cfg *AnalyticsConfig) error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ConnectionUri},
		Auth: clickhouse.Auth{
			Database: "wayfare",
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})

	if err != nil {
		return fmt.Errorf("Failed to connect to analytics provider: %v", err)
	}

	defer conn.Close()

	// Create tables

	w := messaging.NewEventWorker(cfg.Bus, messaging.AnalyticsQueue)
	w.RegisterHandler(HandleTripLifecycleMetrics, messaging.AnalyticsEventTripLifecycle)
	w.RegisterHandler(HandlePaymentMetrics, messaging.AnalyticsEventPayment)

	return w.Start()
}
