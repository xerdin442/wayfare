package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
)

type AnalyticsConfig struct {
	ConnectionUri string
	Username      string
	Password      string
}

func SetupProvider(ctx context.Context, cfg *AnalyticsConfig) (driver.Conn, error) {
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
		return nil, fmt.Errorf("Failed to connect to analytics provider: %v", err)
	}

	if err := models.CreateAnalyticsTable(ctx, conn); err != nil {
		return nil, fmt.Errorf("Failed to create analytics table: %v", err)
	}

	return conn, nil
}

func SendEvent(ctx context.Context, bus messaging.MessageBus, e *models.TripEventModel) error {
	data, err := json.Marshal(messaging.AnalyticsQueuePayload{
		Event: e,
	})
	if err != nil {
		return err
	}

	if err := bus.PublishMessage(
		ctx,
		messaging.AnalyticsExchange,
		messaging.AnalyticsEventUpdate,
		messaging.AmqpMessage{Data: data},
	); err != nil {
		return err
	}

	return nil
}
