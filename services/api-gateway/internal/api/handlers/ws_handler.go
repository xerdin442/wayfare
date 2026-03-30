package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
)

func (h *RouteHandler) monitorConnection(conn *websocket.Conn) {
	conn.SetReadLimit(512)
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		return nil
	})

	go func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}()
}

func (h *RouteHandler) HandleDriversConnection(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userID := c.Query("userID")
	if userID == "" {
		logger.Warn().Msg("User ID not provided")
		return
	}

	h.cfg.ConnManager.Store(userID, conn)
	h.monitorConnection(conn)

	defer func() {
		h.cfg.ConnManager.Delete(userID)
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var payload contracts.WebsocketMessage
		if err := json.Unmarshal(msg, &payload); err != nil {
			logger.Error().Err(err).Msg("Failed to parse incoming websocket message")
			break
		}

		switch payload.Type {
		case messaging.DriverCmdLocationUpdate:
			data := payload.Data.(contracts.DriverLocationUpdateRequest)
			cacheKey := "drivers_locations"

			if err := h.cfg.Cache.GeoAdd(
				c.Request.Context(),
				cacheKey,
				&redis.GeoLocation{
					Name:      userID,
					Longitude: data.Coords.Longitude,
					Latitude:  data.Coords.Latitude,
				},
			).Err(); err != nil {
				logger.Error().Err(err).Msg("Failed to add driver to location tracker")
				return
			}

			if err := h.cfg.Cache.Expire(c.Request.Context(), cacheKey, 15*time.Second).Err(); err != nil {
				logger.Error().Err(err).Msg("Failed to set expiry on location tracker")
				return
			}
		case messaging.DriverCmdTripDecline:
			data := payload.Data.(contracts.DriverTripActionRequest)

			// Find another driver if the previously matched driver declines trip request
			msg, err := json.Marshal(messaging.AssignDriverQueuePayload{
				Trip:     data.Trip,
				DriverID: data.Driver.ID,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.TripEventDriverNotInterested,
				messaging.AmqpMessage{Data: msg},
			); err != nil {
				return
			}
		case messaging.DriverCmdTripAccept:
			payloadData := payload.Data.(contracts.DriverTripActionRequest)

			// Publish to trip service to update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.TripEventDriverAssigned,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return
			}

			// Notify rider that a driver has accepted the trip request
			data, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripEventDriverAssigned,
				Data: contracts.DriverAssignedResponse{
					Driver: payloadData.Driver,
					TripID: payloadData.Trip.ID,
				},
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.UserID)),
				messaging.AmqpMessage{Data: data},
			); err != nil {
				return
			}
		case messaging.DriverCmdTripPickup:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Publish to trip service to update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.DriverCmdTripPickup,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return
			}

			// Notify rider that the driver has arrived
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripEventDriverArrival,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.UserID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				return
			}
		default:
			logger.Warn().Str("message_type", string(payload.Type)).Msg("Unknown websocket message type")
			return
		}
	}
}

func (h *RouteHandler) HandleRidersConnection(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userID := c.Query("userID")
	if userID == "" {
		logger.Warn().Msg("No user ID provided")
		return
	}

	h.cfg.ConnManager.Store(userID, conn)
	h.monitorConnection(conn)

	defer func() {
		h.cfg.ConnManager.Delete(userID)
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var payload contracts.WebsocketMessage
		if err := json.Unmarshal(msg, &payload); err != nil {
			logger.Error().Err(err).Msg("Failed to parse incoming websocket message")
			break
		}

		switch payload.Type {
		case messaging.TripCmdCancelled:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.TripCmdCancelled,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return
			}

			// Publish to gateway to update driver's trip preview
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdCancelled,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.Driver.ID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				return
			}
		case messaging.TripCmdCompleted:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.TripCmdCompleted,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				return
			}

			// Publish to gateway to update driver's trip preview
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdCompleted,
			})
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.Driver.ID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				return
			}
		default:
			logger.Warn().Str("message_type", string(payload.Type)).Msg("Unknown websocket message type")
			return
		}
	}
}
