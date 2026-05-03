package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
)

type AnalyticsEventHandler struct {
	conn clickhouse.Conn
}

func NewAnalyticsEventHandler(conn clickhouse.Conn) *AnalyticsEventHandler {
	return &AnalyticsEventHandler{conn: conn}
}

func (h *AnalyticsEventHandler) HandleAnalyticsEvent(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.AnalyticsQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	ctx = clickhouse.Context(ctx, clickhouse.WithAsync(false))
	e := payload.Event.(models.TripEventModel)

	insertQuery := fmt.Sprintf(
		`INSERT INTO trip_events
		VALUES (%s, %s, %s, %s, %d, %d, %d, %f, %f, %d, %s, %s, %s, %s, %d, %d, %d, %d, now())`,
		e.TripID, e.Region, e.CarPackage, e.TripStatus, e.PredictedDurationMins,
		e.ActualDurationMins, e.DistanceKm, e.PickupLat, e.PickupLng, e.Rating,
		e.TransactionRef, e.DriverID, e.PaymentProvider, e.PaymentStatus,
		e.Amount, e.PlatformFee, e.DriverSplit, e.DriverTip,
	)

	if err := h.conn.Exec(ctx, insertQuery); err != nil {
		return fmt.Errorf("Failed to insert analytics event: %v", err)
	}

	return nil
}
