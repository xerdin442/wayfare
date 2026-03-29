package handlers

import (
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
)

func (h *RouteHandler) HandleDriversConnection(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userID := c.Query("userID")
	packageSlug := c.Query("packageSlug")

	if userID == "" || packageSlug == "" {
		logger.Warn().Msg("User ID or package slug not provided")
		return
	}

	h.cfg.ConnManager.Store(userID, conn)

	defer func() {
		h.cfg.ConnManager.Delete(userID)
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read incoming websocket message")
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

			if err := h.cfg.Cache.Expire(c.Request.Context(), cacheKey, 10*time.Second).Err(); err != nil {
				logger.Error().Err(err).Msg("Failed to set expiry on location tracker")
				return
			}
		case messaging.DriverCmdTripDecline:
			msg := payload.Data.(contracts.DriverTripActionRequest)

			// Find another driver if the previously matched driver declines trip request
			p := messaging.AssignDriverQueuePayload{
				Trip:     msg.Trip,
				DriverID: userID,
			}

			data, err := json.Marshal(p)
			if err != nil {
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				c.Request.Context(),
				messaging.ServicesExchange,
				messaging.TripEventDriverNotInterested,
				messaging.AmqpMessage{Data: data},
			); err != nil {
				return
			}
		case messaging.DriverCmdTripAccept:
			data := payload.Data.(contracts.DriverTripActionRequest)
		case messaging.TripCmdCancelled:
			data := payload.Data.(contracts.RiderTripUpdateRequest)
		case messaging.TripCmdCompleted:
			data := payload.Data.(contracts.RiderTripUpdateRequest)
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

	defer func() {
		h.cfg.ConnManager.Delete(userID)
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read incoming message")
			break
		}

		logger.Printf("Received message: %v", msg)
	}
}
