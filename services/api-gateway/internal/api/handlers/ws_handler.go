package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
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

	h.conns.Store(userID, conn)

	defer func() {
		h.conns.Delete(userID)
		conn.Close()
	}()

	msg := contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: types.Driver{
			ID:             userID,
			Location:       types.Coordinate{},
			Geohash:        "geohash",
			Name:           "Xerdin",
			ProfilePicture: util.GetRandomAvatar(2),
			CarPlate:       "PLATE-123",
		},
	}

	if err := conn.WriteJSON(msg); err != nil {
		logger.Error().Err(err).Msg("Failed to send message")
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			logger.Error().Err(err).Msg("Failed to read incoming message")
			break
		}

		logger.Printf("Received message: %v", msg)
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

	h.conns.Store(userID, conn)

	defer func() {
		h.conns.Delete(userID)
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
