package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
)

type DriverEventsHandler struct {
	repo  *repo.DriverRepository
	cache *redis.Client
	bus   messaging.MessageBus
}

func NewDriverEventsHandler(r *repo.DriverRepository, b messaging.MessageBus, c *redis.Client) *DriverEventsHandler {
	return &DriverEventsHandler{
		repo:  r,
		cache: c,
		bus:   b,
	}
}

func (h *DriverEventsHandler) findAvailableDrivers(ctx context.Context, lat, lng float64) ([]string, error) {
	nearbyDrivers, err := h.cache.GeoSearch(ctx, "drivers_locations", &redis.GeoSearchQuery{
		Longitude:  lng,
		Latitude:   lat,
		Radius:     5,
		RadiusUnit: "km",
	}).Result()

	if err != nil {
		return nil, fmt.Errorf("Failed to find nearby drivers: %v", err)
	}

	return nearbyDrivers, nil
}

func (h *DriverEventsHandler) HandleTripCreated(ctx context.Context, body []byte) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", messaging.TripEventCreated, err)
	}

	var payload messaging.AssignDriverQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", messaging.TripEventCreated, err)
	}

	// Find available drivers within 5km of the trip request
	pickup := payload.Trip.Route.Geometry[0].Coordinates[0]
	drivers, err := h.findAvailableDrivers(ctx, pickup.Latitude, pickup.Longitude)
	if err != nil {
		return err
	}
	targetDriverID := drivers[0]

	if targetDriverID == "" {
		log.Warn().Str("trip_id", payload.Trip.ID).Msg("No drivers available for this trip")

		// Publish to trip service to update trip status
		tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
			TripID: payload.Trip.ID,
		})
		if err != nil {
			return fmt.Errorf("Could not parse event queue payload")
		}

		if err := h.bus.PublishMessage(
			ctx,
			messaging.ServicesExchange,
			messaging.TripEventNoDriversFound,
			messaging.AmqpMessage{Data: tripServiceData},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event", messaging.TripEventNoDriversFound)
		}

		// Publish to gateway to notify user
		gatewayData, err := json.Marshal(messaging.GatewayQueuePayload{
			Type: messaging.TripEventNoDriversFound,
		})
		if err != nil {
			return fmt.Errorf("Could not parse event queue payload")
		}

		if err := h.bus.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", payload.Trip.UserID)),
			messaging.AmqpMessage{Data: gatewayData},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event", messaging.TripEventNoDriversFound)
		}
	}

	// Send trip request to first eligible driver
	gatewayData, err := json.Marshal(messaging.GatewayQueuePayload{
		Type: messaging.DriverEventTripRequest,
		Data: payload.Trip,
	})
	if err != nil {
		return fmt.Errorf("Could not parse event queue payload")
	}

	if err := h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", targetDriverID)),
		messaging.AmqpMessage{Data: gatewayData},
	); err != nil {
		return fmt.Errorf("Failed to publish %s event", messaging.DriverEventTripRequest)
	}

	return nil
}

func (h *DriverEventsHandler) HandleDriverNotInterested(ctx context.Context, body []byte) error

func (h *DriverEventsHandler) HandleDriverNotAvailable(ctx context.Context, body []byte) error
