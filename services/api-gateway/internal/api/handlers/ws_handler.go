package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
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

func (h *RouteHandler) updateDriverStatus(ctx context.Context, driverId string, status types.DriverStatus) {
	data, err := json.Marshal(messaging.DriverUpdateQueuePayload{
		DriverID: driverId,
		Status:   status,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to marshal driver_update queue payload")
		return
	}

	if err := h.cfg.Queue.PublishMessage(
		ctx,
		messaging.ServicesExchange,
		messaging.DriverCmdDetailsUpdate,
		messaging.AmqpMessage{Data: data},
	); err != nil {
		return
	}

	tripEvent := &models.TripEventModel{
		DriverID:     driverId,
		DriverStatus: status,
	}
	if err := analytics.SendEvent(ctx, h.cfg.Queue, tripEvent); err != nil {
		return
	}
}

func (h *RouteHandler) HandleDriversConnection(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleDriversConnection")
	defer span.End()

	logger := log.Ctx(ctx)

	conn, err := h.ws.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("WebSocket connection upgrade failed")
		return
	}

	userId := c.Query("user_id")
	if userId == "" {
		tracing.HandleError(span, fmt.Errorf("user id not provided"))
		logger.Warn().Msg("User ID not provided")
		return
	}

	h.cfg.ConnManager.Store(userId, conn)
	h.updateDriverStatus(ctx, userId, types.DriverStatusOnline)
	h.monitorConnection(conn)

	defer func() {
		h.updateDriverStatus(ctx, userId, types.DriverStatusOffline)
		h.cfg.ConnManager.Delete(userId)
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
			logger.Error().Err(err).Msg("Failed to unmarshal incoming websocket message")
			break
		}

		switch payload.Type {
		case string(messaging.DriverCmdLocationUpdate):
			var data contracts.DriverLocationUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver location_update message")
				continue
			}

			// Send location updates of assigned drivers to riders
			if data.RiderId != "" {
				msg, err = json.Marshal(contracts.WebsocketMessage{
					Type: string(messaging.DriverCmdLocationUpdate),
					Data: types.Coordinate{
						Latitude:  data.Coords.Latitude,
						Longitude: data.Coords.Longitude,
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
					messaging.AmqpEvent(fmt.Sprintf("user.%s", data.RiderId)),
					messaging.AmqpMessage{Data: msg},
				); err != nil {
					tracing.HandleError(span, err)
					return
				}

				return
			}

			// Update location tracker for idle drivers
			if err := h.cfg.Cache.GeoAdd(
				ctx,
				"drivers_locations",
				&redis.GeoLocation{
					Name:      userId,
					Longitude: data.Coords.Longitude,
					Latitude:  data.Coords.Latitude,
				},
			).Err(); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to add driver coordinates to location tracker")
				return
			}

			if err := h.cfg.Cache.Expire(ctx, "drivers_locations", 10*time.Second).Err(); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to set expiry on location tracker")
				return
			}

		case string(messaging.DriverCmdTripDecline):
			var data contracts.DriverTripActionRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver trip_decline message")
				continue
			}

			// Find another driver if the previously matched driver declines trip request
			msg, err := json.Marshal(messaging.AssignDriverQueuePayload{
				Trip:     data.Trip,
				DriverID: data.Driver.ID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripEventDriverNotInterested,
				messaging.AmqpMessage{Data: msg},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.DriverCmdTripAccept):
			var data contracts.DriverTripActionRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver trip_accept message")
				continue
			}

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID:   data.Trip.ID,
				DriverID: data.Driver.ID,
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
				return
			}

			// Notify rider that a driver has accepted the trip request
			msg, err = json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.TripEventDriverAssigned),
				Data: contracts.DriverAssignedResponse{
					Driver: data.Driver,
					Trip:   data.Trip,
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
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.UserID)),
				messaging.AmqpMessage{Data: msg},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

			// Update driver status
			h.updateDriverStatus(ctx, data.Driver.ID, types.DriverStatusBusy)

		case string(messaging.DriverCmdTripPickup):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver trip_pickup message")
				continue
			}

			// Notify rider that the driver has arrived
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.TripEventDriverArrival),
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.UserID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.DriverCmdStartTrip):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver start_trip message")
				continue
			}

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID:    data.Trip.ID,
				StartedAt: time.Now(),
			})

			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdStarted,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

			// Notify rider that the trip has started
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.TripCmdStarted),
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.UserID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.DriverCmdEndTrip):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal driver end_trip message")
				continue
			}

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID:  data.Trip.ID,
				EndedAt: time.Now(),
			})

			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.DriverCmdEndTrip,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

			// Notify rider to make payment
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.TripEventPaymentRequired),
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.UserID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.PaymentEventCashReceived):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal cash payment request message")
				continue
			}

			// Get trip details
			tripDetails, err := h.cfg.Clients.Trip.GetTripDetails(ctx, &pb.TripDetailsRequest{
				TripId: data.Trip.ID,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to get trip details")
				return
			}

			// Send payment details to payment service
			paymentServiceData, err := json.Marshal(messaging.CashPaymentPayload{
				TripID:  data.Trip.ID,
				RiderID: tripDetails.UserId,
				Amount:  tripDetails.RideFare,
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

	userID := c.Query("user_id")
	if userID == "" {
		tracing.HandleError(span, fmt.Errorf("user id not provided"))
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
			logger.Error().Err(err).Msg("Failed to unmarshal incoming websocket message")
			break
		}

		switch payload.Type {
		case string(messaging.TripCmdCancelled):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal trip_update request message")
				continue
			}

			// Update trip status
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID: data.Trip.ID,
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
				return
			}

			// Update driver's trip preview
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.TripCmdCancelled),
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.DriverID)),

				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.TripCmdRated):
			var data contracts.TripRatingRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal trip_update request message")
				continue
			}

			// Update trip rating
			tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
				TripID:       data.TripId,
				Rating:       data.Rating,
				RiderComment: data.RiderComment,
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal trip_update queue payload")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.ServicesExchange,
				messaging.TripCmdRated,
				messaging.AmqpMessage{Data: tripServiceData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		case string(messaging.PaymentEventCashOptionPreferred):
			var data contracts.TripUpdateRequest
			dataBytes, _ := json.Marshal(payload.Data)
			if err := json.Unmarshal(dataBytes, &data); err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to unmarshal trip_update request message")
				continue
			}

			// Notify driver that rider prefers cash payment
			gatewayData, err := json.Marshal(contracts.WebsocketMessage{
				Type: string(messaging.PaymentEventCashOptionPreferred),
			})
			if err != nil {
				tracing.HandleError(span, err)
				logger.Error().Err(err).Msg("Failed to marshal websocket message")
				return
			}

			if err := h.cfg.Queue.PublishMessage(
				ctx,
				messaging.GatewayExchange,
				messaging.AmqpEvent(fmt.Sprintf("user.%s", data.Trip.DriverID)),
				messaging.AmqpMessage{Data: gatewayData},
			); err != nil {
				tracing.HandleError(span, err)
				return
			}

		default:
			logger.Warn().Str("message_type", string(payload.Type)).Msg("Unknown websocket message type")
			return
		}
	}
}
