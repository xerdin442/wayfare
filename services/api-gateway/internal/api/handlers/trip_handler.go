package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *RouteHandler) HandleTripPreview(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleTripPreview")
	defer span.End()

	logger := log.Ctx(ctx)

	userID := c.MustGet("user_id").(string)

	var req contracts.PreviewTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error parsing preview trip request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Trip.PreviewTrip(ctx, &pb.PreviewTripRequest{
		UserId:      userID,
		Pickup:      req.ToProto().Pickup,
		Destination: req.ToProto().Destination,
	})

	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error requesting trip preview")

		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument, codes.FailedPrecondition:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request trip preview"})
			}

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request trip preview"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandleStartTrip(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleStartTrip")
	defer span.End()

	logger := log.Ctx(ctx)

	userID := c.MustGet("user_id").(string)

	var req contracts.StartTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Start trip request failed")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Trip.StartTrip(ctx, &pb.StartTripRequest{
		UserId:     userID,
		RideFareId: req.RideFareId,
	})

	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Start trip request failed")

		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.InvalidArgument:
				c.JSON(http.StatusBadRequest, gin.H{"error": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occured while starting the trip"})
			}

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occured while starting the trip"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandleInitiatePayment(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleInitiatePayment")
	defer span.End()

	logger := log.Ctx(ctx)

	userId := c.MustGet("user_id").(string)
	tripId := c.Param("id")

	var req contracts.InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error parsing initiate payment request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := fmt.Sprintf("lock:payment:%s", tripId)

	// Check if request is still being processed
	n, err := h.cfg.Cache.Exists(ctx, idempotencyKey).Result()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error fetching idempotency lock from cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while processing payment"})
		return
	}

	if n > 0 {
		tracing.HandleError(span, fmt.Errorf("payment request is already being processed"))
		c.JSON(http.StatusConflict, gin.H{"error": "Payment request is already being processed"})
		return
	}

	// Set idempotency lock in cache
	if err := h.cfg.Cache.Set(ctx, idempotencyKey, "locked", 2*time.Minute).Err(); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error setting idempotency lock")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while processing payment"})
		return
	}

	// Generate checkout link
	checkoutResponse, err := h.cfg.Clients.Payment.InitiatePayment(ctx, &pb.InitiatePaymentRequest{
		TripId:         tripId,
		UserId:         userId,
		Email:          req.Email,
		CustomRedirect: req.CustomRedirect,
		TripRating:     req.TripRating,
		RiderComment:   req.RiderComment,
		DriverTip:      req.DriverTip,
	})
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Str("trip_id", tripId).Msg("Failed to generate checkout url")

		// Remove idempotency lock if payment request fails
		if err := h.cfg.Cache.Del(ctx, idempotencyKey).Err(); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Error removing idempotency lock")
		}

		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"error": st.Message()})
			case codes.Unavailable:
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while processing payment"})
			}

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while processing payment"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: checkoutResponse})
}
