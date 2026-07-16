package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
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

		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Validation failed",
				"errors":  util.FormatValidationErrors(err, &req),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
		return
	}

	response, err := h.cfg.Clients.Trip.PreviewTrip(ctx, &pb.PreviewTripRequest{
		UserId:      userID,
		RegionId:    req.RegionId,
		Pickup:      req.ToProto().Pickup,
		Destination: req.ToProto().Destination,
	})

	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error requesting trip preview")

		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.FailedPrecondition:
				c.JSON(http.StatusBadRequest, gin.H{"message": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request trip preview"})
			}

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request trip preview"})
		return
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{
			"rideFares": contracts.MapPbRideFares(response.RideFares),
		},
	})
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

		var ve validator.ValidationErrors
		if errors.As(err, &ve) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Validation failed",
				"errors":  util.FormatValidationErrors(err, &req),
			})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request"})
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
				c.JSON(http.StatusBadRequest, gin.H{"message": st.Message()})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occured while starting the trip"})
			}

			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occured while starting the trip"})
		return
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{
			"tripId": response.TripId,
		},
	})
}

func (h *RouteHandler) HandleTripChat(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleTripChat")
	defer span.End()

	logger := log.Ctx(ctx)

	tripId := c.Param("id")

	result, err := h.cfg.Cache.LRange(ctx, fmt.Sprintf("trip_chat_history:%s", tripId), 0, -1).Result()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Str("trip_id", tripId).Msg("Failed to fetch trip chat history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while fetching trip chat history"})
		return
	}

	chatHistory := make([]types.ChatMessage, 0, len(result))
	for _, item := range result {
		var chatObj types.ChatMessage

		if err := json.Unmarshal([]byte(item), &chatObj); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Failed to parse chat history")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while fetching trip chat history"})
			return
		}

		chatHistory = append(chatHistory, chatObj)
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{
			"history": chatHistory,
		},
	})
}

func (h *RouteHandler) HandleTripHistory(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleTripHistory")
	defer span.End()

	logger := log.Ctx(ctx)

	userId := c.MustGet("user_id").(string)

	response, err := h.cfg.Clients.Trip.GetTripHistory(ctx, &pb.TripHistoryRequest{
		UserId: userId,
	})

	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to fetch trip history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while fetching trip history"})
		return
	}

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{
			"trips": contracts.MapPbTrips(response.Trips),
		},
	})
}
