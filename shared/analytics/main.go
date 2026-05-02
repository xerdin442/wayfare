package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
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

	if err := CreateAnalyticsTable(ctx, conn); err != nil {
		return fmt.Errorf("Failed to create analytics table: %v", err)
	}

	h := NewAnalyticsEventHandler(conn)
	w := messaging.NewEventWorker(cfg.Bus, messaging.AnalyticsQueue)
	w.RegisterHandler(h.HandleAnalyticsEvent, messaging.AnalyticsEventUpdate)

	return w.Start()
}

func SendEvent(ctx context.Context, bus messaging.MessageBus, e *models.TripEventModel) error {
	data, err := json.Marshal(messaging.AnalyticsQueuePayload{
		Event: e,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal analytics queue payload")
	}

	if err := bus.PublishMessage(
		ctx,
		messaging.AnalyticsExchange,
		messaging.AnalyticsEventUpdate,
		messaging.AmqpMessage{Data: data},
	); err != nil {
		return fmt.Errorf("Failed to publish analytics event")
	}

	return nil
}
