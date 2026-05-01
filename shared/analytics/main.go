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

func SetupProvider(ctx context.Context, cfg *AnalyticsConfig) error {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ConnectionUri},
		Auth: clickhouse.Auth{
			Database: "wayfare",
			Username: cfg.Username,
			Password: cfg.Password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time":    60,
			"async_insert":          1,
			"wait_for_async_insert": 1,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})

	if err != nil {
		return fmt.Errorf("Failed to connect to analytics provider: %v", err)
	}

	defer conn.Close()

	if err := CreateAnalyticsTables(ctx, conn); err != nil {
		return fmt.Errorf("Failed to create analytics event tables: %v", err)
	}

	h := NewAnalyticsEventHandler(conn)
	w := messaging.NewEventWorker(cfg.Bus, messaging.AnalyticsQueue)

	w.RegisterHandler(
		h.HandleAnalyticsEvent,
		messaging.AnalyticsEventTripLifecycle,
		messaging.AnalyticsEventPayment,
	)

	return w.Start()
}
