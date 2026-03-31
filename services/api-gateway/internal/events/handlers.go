package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/types"
)

type GatewayEventsHandler struct {
	cfg *base.Config
}

func NewGatewayEventsHandler(c *base.Config) *GatewayEventsHandler {
	return &GatewayEventsHandler{cfg: c}
}

func (h *GatewayEventsHandler) HandleGatewayQueueEvents(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from gateway queue: %v", err)
	}

	var payload contracts.WebsocketMessage
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	userID := strings.TrimPrefix(p.RoutingKey, "user.")
	val, exists := h.cfg.ConnManager.Load(userID)

	if !exists {
		switch payload.Type {
		case messaging.DriverEventTripRequest:
			// Find another driver if the previously matched driver is offline
			data, err := json.Marshal(messaging.AssignDriverQueuePayload{
				Trip: payload.Data.(types.Trip),
			})
			if err != nil {
				return fmt.Errorf("Could not marshal payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverNotAvailable,
				messaging.AmqpMessage{Data: data},
			); err != nil {
				return fmt.Errorf("Failed to publish %s event: %v", messaging.TripEventDriverNotAvailable, err)
			}
		case messaging.TripEventNoDriversFound:
			return nil
		case messaging.TripEventDriverAssigned:
			response := payload.Data.(contracts.DriverAssignedResponse)

			// Abort trip if the rider is offline when a driver accepts the trip request
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: response.Trip.ID,
			})
			if err != nil {
				return fmt.Errorf("Could not marshal payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdAborted,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return fmt.Errorf("Failed to publish %s event: %v", messaging.TripCmdAborted, err)
			}

			// Notify the driver that the trip has been aborted
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdAborted,
			})
			if err != nil {
				return fmt.Errorf("Could not marshal payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", response.Driver.ID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				return fmt.Errorf("Failed to publish %s event: %v", messaging.TripCmdAborted, err)
			}
		default:
			return fmt.Errorf("Unknown payload event type received by gateway queue: %s", payload.Type)
		}
	}

	conn, _ := val.(*websocket.Conn)
	if err := conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("Failed to send websocket message: %v", err)
	}

	return nil
}
