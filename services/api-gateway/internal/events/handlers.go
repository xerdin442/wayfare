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

func (h *GatewayEventsHandler) HandleOutgoingWebsocketMessages(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from gateway queue: %v", err)
	}

	var payload contracts.WebsocketMessage
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	userId := strings.TrimPrefix(p.RoutingKey, "user.")
	val, exists := h.cfg.ConnManager.Load(userId)

	if !exists {
		switch payload.Type {
		case messaging.DriverEventTripRequest:
			var response types.Trip
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &response); err != nil {
				return fmt.Errorf("Failed to unmarshal trip_request response: %v", err)
			}

			// Find another driver if the previously matched driver is offline
			data, err := json.Marshal(messaging.AssignDriverQueuePayload{
				Trip: response,
			})
			if err != nil {
				return fmt.Errorf("Failed to marshal assign_driver queue payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverNotAvailable,
				messaging.AmqpMessage{Data: data},
			); err != nil {
				return err
			}
		case messaging.TripEventDriverAssigned:
			var response contracts.DriverAssignedResponse
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &response); err != nil {
				return fmt.Errorf("Failed to unmarshal driver_assigned response: %v", err)
			}

			// Abort trip if the rider is offline when a driver accepts the trip request
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: response.Trip.ID,
			})
			if err != nil {
				return fmt.Errorf("Failed to marshal trip_update queue payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdAborted,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return err
			}

			// Notify the driver that the trip has been aborted
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdAborted,
			})
			if err != nil {
				return fmt.Errorf("Failed to marshal websocket message: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", response.Driver.ID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				return err
			}

			// Update driver status
			driverServiceData, err := json.Marshal(messaging.DriverUpdateQueuePayload{
				DriverID: response.Driver.ID,
				Status:   types.DriverStatusOnline,
			})
			if err != nil {
				return fmt.Errorf("Failed to marshal driver_update queue payload: %v", err)
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.DriverCmdDetailsUpdate,
				messaging.AmqpMessage{Data: driverServiceData},
			); err != nil {
				return err
			}

		default:
			return fmt.Errorf("Invalid user ID received by gateway queue: %s", userId)
		}
	}

	conn, _ := val.(*websocket.Conn)
	if err := conn.WriteJSON(payload); err != nil {
		return fmt.Errorf("Failed to send websocket message: %v", err)
	}

	return nil
}
