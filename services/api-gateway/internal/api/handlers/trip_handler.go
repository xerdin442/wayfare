package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
)

var (
	ErrRequestAlreadyProcessed = errors.New("Request is already being processed")
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
		logger.Error().Err(err).Msg("Error parsing start trip request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Trip.StartTrip(ctx, &pb.StartTripRequest{
		UserId:     userID,
		RideFareId: req.RideFareID,
	})

	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Start trip request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start trip"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandleInitiatePayment(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleInitiatePayment")
	defer span.End()

	logger := log.Ctx(ctx)

	userID := c.MustGet("user_id").(string)
	tripID := c.Param("id")

	var req contracts.InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error parsing initiate payment request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	idempotencyKey := fmt.Sprintf("lock:payment:%s", tripID)

	// Check if request is still being processed
	n, err := h.cfg.Cache.Exists(ctx, idempotencyKey).Result()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error fetching idempotency lock from cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	if n > 0 {
		tracing.HandleError(span, ErrRequestAlreadyProcessed)
		c.JSON(http.StatusConflict, gin.H{"error": "Request is already being processed"})
		return
	}

	// Set idempotency lock in cache
	if err := h.cfg.Cache.Set(ctx, idempotencyKey, "locked", 2*time.Minute).Err(); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error setting idempotency lock")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	// Verify trip details
	tripDetails, err := h.cfg.Clients.Trip.GetTripDetails(ctx, &pb.TripDetailsRequest{
		TripId: tripID,
	})
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Str("trip_id", tripID).Msg("Failed to get trip details")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	if tripDetails.UserId != userID {
		log.Warn().
			Str("trip_owner_id", tripDetails.UserId).
			Str("payment_request_initiator_id", userID).
			Msg("Unauthorized access")

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized access"})
		return
	}

	// Generate checkout link
	checkoutResponse, err := h.cfg.Clients.Payment.InitiatePayment(ctx, &pb.InitiatePaymentRequest{
		TripId:         tripID,
		Email:          req.Email,
		Amount:         tripDetails.RideFareAmount,
		CustomRedirect: req.CustomRedirect,
	})
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Str("trip_id", tripID).Msg("Failed to initiate processing of payment request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: checkoutResponse})
}
