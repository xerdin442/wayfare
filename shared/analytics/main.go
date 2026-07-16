package analytics

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
)

type AnalyticsConfig struct {
	ConnectionUri string
	Username      string
	Password      string
}

func SetupProvider(ctx context.Context, cfg *AnalyticsConfig) (driver.Conn, error) {
	opts := &clickhouse.Options{
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
	}

	var conn driver.Conn
	var err error

	for range 3 {
		conn, err = clickhouse.Open(opts)
		if err == nil {
			break
		}

		log.Warn().Msg("Waiting for analytics provider...")
		time.Sleep(time.Second * 5)
	}

	if err != nil {
		return nil, fmt.Errorf("Could not connect to analytics provider after 3 attempts. %v", err)
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
