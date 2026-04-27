package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
)

var (
	ErrMissingUserId = errors.New("User ID not provided")
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
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleDriversConnection")
	defer span.End()

	logger := log.Ctx(ctx)

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		tracing.HandleError(span, err)
		log.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userID := c.Query("userID")
	if userID == "" {
		tracing.HandleError(span, ErrMissingUserId)
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
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse incoming websocket message")
			break
		}

		switch payload.Type {
		case messaging.DriverCmdLocationUpdate:
			data := payload.Data.(contracts.DriverLocationUpdateRequest)
			cacheKey := "drivers_locations"

			if err := h.cfg.Cache.GeoAdd(
				ctx,
				cacheKey,
				&redis.GeoLocation{
					Name:      userID,
					Longitude: data.Coords.Longitude,
					Latitude:  data.Coords.Latitude,
				},
			).Err(); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to add driver coordinates to location tracker")
				return
			}

			if err := h.cfg.Cache.Expire(ctx, cacheKey, 10*time.Second).Err(); err != nil {
				tracing.HandleError(span, err)
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
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal assign_driver queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverNotInterested,
				messaging.AmqpMessage{Data: msg},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.TripEventDriverNotInterested)
				return
			}
		case messaging.DriverCmdTripAccept:
			payloadData := payload.Data.(contracts.DriverTripActionRequest)

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID:   payloadData.Trip.ID,
				DriverID: payloadData.Driver.ID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverAssigned,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.TripEventDriverAssigned)
				return
			}

			// Notify rider that a driver has accepted the trip request
			data, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripEventDriverAssigned,
				Data: contracts.DriverAssignedResponse{
					Driver: payloadData.Driver,
					Trip:   payloadData.Trip,
				},
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.UserID)),
				messaging.AmqpMessage{Data: data},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to publish gateway event")
				return
			}
		case messaging.DriverCmdTripPickup:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.DriverCmdTripPickup,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.DriverCmdTripPickup)
				return
			}

			// Notify rider that the driver has arrived
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripEventDriverArrival,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.UserID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to publish gateway event")
				return
			}
		case messaging.PaymentEventCashReceived:
			payloadData := payload.Data.(contracts.CashPaymentRequest)

			// Get trip details
			tripDetails, err := h.cfg.Clients.Trip.GetTripDetails(ctx, &rpc.TripDetailsRequest{
				TripId: payloadData.TripID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to get trip details")
				return
			}

			// Send details of cash payment to payment service
			paymentServiceData, err := json.Marshal(messaging.PaymentQueuePayload{
				TripID: payloadData.TripID,
				Amount: tripDetails.RideFareAmount,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal payment queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.PaymentEventCashReceived,
				messaging.AmqpMessage{Data: paymentServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.PaymentEventCashReceived)
				return
			}
		default:
			logger.Warn().Str("message_type", string(payload.Type)).Msg("Unknown websocket message type")
			return
		}
	}
}

func (h *RouteHandler) HandleRidersConnection(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleRidersConnection")
	defer span.End()

	logger := log.Ctx(ctx)

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userID := c.Query("userID")
	if userID == "" {
		tracing.HandleError(span, ErrMissingUserId)
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
			tracing.HandleError(span, err)
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
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdCancelled,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.TripCmdCancelled)
				return
			}

			// Update driver's trip preview
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdCancelled,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.DriverID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to publish gateway event")
				return
			}
		case messaging.TripCmdCompleted:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: payloadData.Trip.ID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdCompleted,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.TripCmdCompleted)
				return
			}

			// Update driver's trip preview
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.TripCmdCompleted,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.DriverID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to publish gateway event")
				return
			}
		case messaging.PaymentEventCashOptionPreferred:
			payloadData := payload.Data.(contracts.TripUpdateRequest)

			// Notify driver that rider prefers cash payment
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: messaging.PaymentEventCashOptionPreferred,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", payloadData.Trip.DriverID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to publish gateway event")
				return
			}
		default:
			logger.Warn().Str("message_type", string(payload.Type)).Msg("Unknown websocket message type")
			return
		}
	}
}
