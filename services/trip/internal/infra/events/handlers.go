package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/shopspring/decimal"
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/types"
)

type TripEventsHandler struct {
	repo *repo.TripRepository
	bus  messaging.MessageBus
}

func NewTripEventsHandler(r *repo.TripRepository, b messaging.MessageBus) *TripEventsHandler {
	return &TripEventsHandler{
		repo: r,
		bus:  b,
	}
}

func (h *TripEventsHandler) HandleTripUpdate(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.TripUpdateQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	var updatedStatus types.TripStatus

	switch messaging.AmqpEvent(p.RoutingKey) {
	case messaging.TripEventNoDriversFound:
	case messaging.TripCmdAborted:
		updatedStatus = types.TripStatusAborted
	case messaging.TripEventDriverAssigned:
		updatedStatus = types.TripStatusMatched
	case messaging.DriverCmdTripPickup:
		updatedStatus = types.TripStatusActive
	case messaging.DriverCmdEndTrip:
		updatedStatus = types.TripStatusAwaitingPayment
	case messaging.TripCmdCancelled:
		updatedStatus = types.TripStatusCancelled
	case messaging.TripCmdCompleted:
		updatedStatus = types.TripStatusCompleted
	default:
		return fmt.Errorf("Unknown event type received in trip update queue: %s", p.RoutingKey)
	}

	updateData := &repo.TripUpdateData{
		NewStatus:    updatedStatus,
		DriverID:     payload.DriverID,
		PickupAt:     payload.PickupAt,
		EndedAt:      payload.EndedAt,
		Rating:       payload.Rating,
		RiderComment: payload.RiderComment,
	}

	updatedTrip, err := h.repo.UpdateTrip(ctx, payload.TripID, updateData)
	if err != nil {
		return err
	}

	if updatedStatus == types.TripStatusCompleted {
		var publishErr error

		// Notify participants that the trip has ended
		gatewayData, err := json.Marshal(contracts.WebsocketMessage{
			Type: messaging.TripCmdCompleted,
		})
		if err != nil {
			return fmt.Errorf("Failed to marshal websocket message")
		}

		publishErr = h.bus.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", updatedTrip.DriverID.Hex())),
			messaging.AmqpMessage{Data: gatewayData},
		)

		publishErr = h.bus.PublishMessage(
			ctx,
			messaging.GatewayExchange,
			messaging.AmqpEvent(fmt.Sprintf("user.%s", updatedTrip.UserID.Hex())),
			messaging.AmqpMessage{Data: gatewayData},
		)

		if publishErr != nil {
			return fmt.Errorf("Failed to publish gateway event")
		}

		// Update driver completed trips count
		tripServiceData, err := json.Marshal(messaging.DriverUpdateQueuePayload{
			DriverID:        payload.DriverID,
			TripCountUpdate: true,
		})
		if err != nil {
			return fmt.Errorf("Failed to marshal driver_update queue payload: %v", err)
		}

		if err = h.bus.PublishMessage(
			ctx,
			messaging.ServicesExchange,
			messaging.DriverCmdTripCountUpdate,
			messaging.AmqpMessage{Data: tripServiceData},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event: %v", messaging.DriverCmdTripCountUpdate, err)
		}
	}

	actualDuration := payload.EndedAt.Sub(payload.PickupAt).Minutes()
	tripEvent := &models.TripEventModel{
		TripID:             payload.TripID,
		DriverID:           payload.DriverID,
		TripStatus:         updatedStatus,
		Rating:             payload.Rating,
		ActualDurationMins: decimal.NewFromFloat(actualDuration),
	}
	if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
		return err
	}

	return nil
}
