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

	var payload contracts.WSOutgoingMessage
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from gateway queue: %v", err)
	}

	userID := strings.TrimPrefix(p.RoutingKey, "user.")
	val, exists := h.cfg.ConnManager.Load(userID)

	if !exists {
		switch payload.Type {
		case messaging.DriverEventTripRequest:
			// Find another driver if the previously matched driver is offline
			p := messaging.AssignDriverQueuePayload{
				Trip: payload.Data.(types.Trip),
			}

			data, err := json.Marshal(p)
			if err != nil {
				return fmt.Errorf("Could not marshal payload")
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverNotAvailable,
				messaging.AmqpMessage{Data: data},
			); err != nil {
				return fmt.Errorf("Failed to publish %s event", messaging.TripEventDriverNotAvailable)
			}
		case messaging.TripEventNoDriversFound:
			return nil
		default:
			return fmt.Errorf("Unknown payload event type received by gateway queue: %s", payload.Type)
		}
	}

	conn, _ := val.(*websocket.Conn)
	if err := conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("Failed to send websocket message")
	}

	return nil
}
