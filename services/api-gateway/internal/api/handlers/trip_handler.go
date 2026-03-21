package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
)

func (h *RouteHandler) HandleTripPreview(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	// userID := c.MustGet("userID").(int32)

	var req contracts.PreviewTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Trip preview request error")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

		return
	}
}
