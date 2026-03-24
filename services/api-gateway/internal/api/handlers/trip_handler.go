package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
)

func (h *RouteHandler) HandleTripPreview(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	userID := c.MustGet("user_id").(string)

	var req contracts.PreviewTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Error parsing preview trip request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Trip.PreviewTrip(c.Request.Context(), &rpc.PreviewTripRequest{
		UserId:      userID,
		Pickup:      req.ToProto().Pickup,
		Destination: req.ToProto().Destination,
	})

	if err != nil {
		logger.Error().Err(err).Msg("Error requesting trip preview")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to request trip preview"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandleStartTrip(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	userID := c.MustGet("user_id").(string)

	var req contracts.StartTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Error parsing start trip request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Trip.StartTrip(c.Request.Context(), &rpc.StartTripRequest{
		UserId:     userID,
		RideFareId: req.RideFareID,
	})

	if err != nil {
		logger.Error().Err(err).Msg("Start trip request failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start trip"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}
