package events

import (
	"context"
	"encoding/json"
	"fmt"

	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
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

	if err := h.repo.UpdateTrip(ctx, payload.TripID, updateData); err != nil {
		return err
	}

	return nil
}
