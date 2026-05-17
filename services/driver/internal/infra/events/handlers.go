package events

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/types"
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

func (h *DriverEventsHandler) calculateDriverSplit(ctx context.Context, driverID string, rideFare int64) (int64, error) {
	driver, err := h.repo.GetDriverByID(ctx, driverID)
	if err != nil {
		return 0, err
	}

	var splitRate float64
	switch driver.Tier {
	case types.TierGold:
		splitRate = 0.88
	case types.TierSilver:
		splitRate = 0.85
	case types.TierBronze:
		splitRate = 0.80
	default:
		splitRate = 0.80
	}

	driverSplit := int64(float64(rideFare) * splitRate)
	return ((driverSplit + 5) / 10) * 10, nil
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
	pickup := payload.Trip.SelectedFare.Route.Geometry[0].Coordinates[0]
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

	// Blacklist uninterested driver
	cacheKey := fmt.Sprintf("blacklisted_drivers:%s", payload.Trip.ID)
	if p.RoutingKey == string(messaging.TripEventDriverNotInterested) {
		if err := h.cache.SAdd(ctx, cacheKey, payload.DriverID).Err(); err != nil {
			return fmt.Errorf("Failed to blacklist uninterested driver: %v", err)
		}

		if err := h.cache.Expire(ctx, cacheKey, 15*time.Minute).Err(); err != nil {
			return fmt.Errorf("Failed to set expiry on blacklisted drivers: %v", err)
		}
	}

	// Fetch blacklisted drivers
	blacklistedDrivers, err := h.cache.SMembers(ctx, cacheKey).Result()
	if err != nil && err != redis.Nil {
		log.Error().Err(err).Msg("Failed to fetch blacklisted drivers")
	}

	// Filter nearby drivers
	var eligibleDrivers []string
	for _, id := range nearbyDrivers {
		if !slices.Contains(blacklistedDrivers, id) {
			eligibleDrivers = append(eligibleDrivers, id)
		}
	}

	// Find matching driver
	targetDriverID := h.repo.GetDriverByPackage(ctx, eligibleDrivers, payload.Trip.SelectedFare.PackageSlug)

	// No drivers found
	if targetDriverID == "" {
		log.Warn().Str("trip_id", payload.Trip.ID).Msg("No drivers available for this trip")

		// Update trip status
		tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
			TripID: payload.Trip.ID,
		})
		if err != nil {
			return fmt.Errorf("Failed to marshal trip_update queue payload: %v", err)
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
			Type: string(messaging.TripEventNoDriversFound),
		})
		if err != nil {
			return fmt.Errorf("Failed to marshal websocket message: %v", err)
		}

		if err := h.bus.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", payload.Trip.UserID)),
			messaging.AmqpMessage{Data: gatewayData},
		); err != nil {
			return fmt.Errorf("Failed to publish gateway event: %v", err)
		}

		return nil
	}

	// Send trip request to eligible driver
	data, err := json.Marshal(contracts.WebsocketMessage{
		Type: string(messaging.DriverEventTripRequest),
		Data: payload.Trip,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal websocket message: %v", err)
	}

	if err := h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", targetDriverID)),
		messaging.AmqpMessage{Data: data},
	); err != nil {
		return fmt.Errorf("Failed to publish gateway event: %v", err)
	}

	return nil
}

func (h *DriverEventsHandler) HandleDriverUpdate(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.DriverUpdateQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	splitAmount, err := h.calculateDriverSplit(ctx, payload.DriverID, payload.RideFare)
	if err != nil {
		return err
	}

	if payload.RecipientCode != "" {
		if err := h.repo.ResetPendingPayout(ctx, payload.RecipientCode); err != nil {
			return err
		}
	} else {
		updateData := &repo.DriverUpdateData{
			Status:                  payload.Status,
			SplitAmount:             splitAmount + payload.Tip,
			TripCountUpdate:         payload.TripCountUpdate,
			BalanceUpdate:           payload.BalanceUpdate,
			PendingReturnsUpdate:    payload.PendingReturnsUpdate,
			OutstandingReturnsReset: payload.OutstandingReturnsReset,
		}
		if err := h.repo.UpdateDriverDetails(ctx, payload.DriverID, updateData); err != nil {
			return err
		}

		if !payload.OutstandingReturnsReset {
			tripEvent := &models.TripEventModel{
				DriverID:      payload.DriverID,
				DriverSplit:   decimal.NewFromInt(splitAmount / 100),
				PlatformSplit: decimal.NewFromInt((payload.RideFare - splitAmount) / 100),
			}
			if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
				return err
			}
		}
	}

	return nil
}
