package analytics

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/xerdin442/wayfare/shared/messaging"
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
	e := payload.Event

	ctx = clickhouse.Context(ctx, clickhouse.WithAsync(false))

	insertQuery := `
		INSERT INTO trip_events (
			trip_id, region, car_package, trip_status, predicted_duration_mins,
			actual_duration_mins, distance_km, pickup_lat, pickup_lng, rating,
			transaction_ref, transaction_type, driver_id, payment_provider, payment_status,
			amount, platform_fee, driver_split, driver_tip, timestamp
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, now())
	`

	if err := h.conn.Exec(ctx, insertQuery,
		e.TripID, e.Region, e.CarPackage, e.TripStatus, e.PredictedDurationMins,
		e.ActualDurationMins, e.DistanceKm, e.PickupLat, e.PickupLng, e.Rating,
		e.TransactionRef, e.TransactionType, e.DriverID, e.PaymentProvider, e.PaymentStatus,
		e.Amount, e.PlatformFee, e.DriverSplit, e.DriverTip,
	); err != nil {
		return fmt.Errorf("Failed to insert analytics event: %v", err)
	}

	return nil
}
