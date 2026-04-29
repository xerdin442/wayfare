package events

import (
	"context"
	"encoding/json"
	"fmt"

	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
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
		Rating:       payload.Rating,
		RiderComment: payload.RiderComment,
	}

	updatedTrip, err := h.repo.UpdateTrip(ctx, payload.TripID, updateData)
	if err != nil {
		return err
	}

	// Notify participants that the trip has ended
	if updatedStatus == types.TripStatusCompleted {
		if err := h.sendTripCompletionStatus(ctx, updatedTrip.DriverID.Hex(), updatedTrip.UserID.Hex()); err != nil {
			return err
		}
	}

	return nil
}

func (h *TripEventsHandler) sendTripCompletionStatus(ctx context.Context, driverID, riderID string) error {
	var publishErr error

	gatewayData, err := json.Marshal(contracts.WebsocketMessage{
		Type: messaging.TripCmdCompleted,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal websocket message")
	}

	// Update the driver's trip preview
	publishErr = h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", driverID)),
		messaging.AmqpMessage{Data: gatewayData},
	)

	// Update the rider's trip preview
	publishErr = h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", riderID)),
		messaging.AmqpMessage{Data: gatewayData},
	)

	return publishErr
}
