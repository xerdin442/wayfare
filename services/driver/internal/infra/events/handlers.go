package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/contracts"
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

func (h *DriverEventsHandler) FindAndAssignDriver(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.AssignDriverQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	// Find available drivers within 5km of the trip request
	pickup := payload.Trip.Route.Geometry[0].Coordinates[0]
	nearbyDrivers, err := h.cache.GeoSearch(ctx, "drivers_locations", &redis.GeoSearchQuery{
		Longitude:  pickup.Longitude,
		Latitude:   pickup.Latitude,
		Radius:     5,
		RadiusUnit: "km",
		Sort:       "ASC",
	}).Result()
	if err != nil {
		return fmt.Errorf("Failed to find nearby drivers: %v", err)
	}

	var targetDriverID string
	targetDriverID = nearbyDrivers[0]

	// Blacklist uninterested driver
	if p.RoutingKey == string(messaging.TripEventDriverNotInterested) {
		cacheKey := fmt.Sprintf("blacklisted_drivers:%s", payload.Trip.ID)

		if err := h.cache.SAdd(ctx, cacheKey, payload.DriverID).Err(); err != nil {
			return fmt.Errorf("Failed to blacklist uninterested driver: %v", err)
		}

		if err := h.cache.Expire(ctx, cacheKey, 15*time.Minute).Err(); err != nil {
			return fmt.Errorf("Failed to set expiry on blacklisted drivers: %v", err)
		}

		for _, driverID := range nearbyDrivers {
			if driverID != payload.DriverID {
				targetDriverID = driverID
				break
			}
		}
	}

	if targetDriverID == "" {
		log.Warn().Str("trip_id", payload.Trip.ID).Msg("No drivers available for this trip")

		// Publish to trip service to update trip status
		tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
			TripID: payload.Trip.ID,
		})
		if err != nil {
			return fmt.Errorf("Could not parse event queue payload: %v", err)
		}

		if err := h.bus.PublishMessage(
			ctx,
			messaging.ServicesExchange,
			messaging.TripEventNoDriversFound,
			messaging.AmqpMessage{Data: tripServiceData},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event: %v", messaging.TripEventNoDriversFound, err)
		}

		// Notify the rider there are no available drivers
		gatewayData, err := json.Marshal(contracts.WebsocketMessage{
			Type: messaging.TripEventNoDriversFound,
		})
		if err != nil {
			return fmt.Errorf("Could not parse event queue payload: %v", err)
		}

		if err := h.bus.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", payload.Trip.UserID)),
			messaging.AmqpMessage{Data: gatewayData},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event: %v", messaging.TripEventNoDriversFound, err)
		}
	}

	// Send trip request to eligible driver
	data, err := json.Marshal(contracts.WebsocketMessage{
		Type: messaging.DriverEventTripRequest,
		Data: payload.Trip,
	})
	if err != nil {
		return fmt.Errorf("Could not parse event queue payload: %v", err)
	}

	if err := h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", targetDriverID)),
		messaging.AmqpMessage{Data: data},
	); err != nil {
		return fmt.Errorf("Failed to publish %s event: %v", messaging.DriverEventTripRequest, err)
	}

	return nil
}
