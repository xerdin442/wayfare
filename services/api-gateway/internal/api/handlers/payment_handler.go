package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
)

func (h *RouteHandler) HandleInitiatePayment(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	var req contracts.InitiatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error().Err(err).Msg("Error parsing initiate payment request")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.cfg.Clients.Payment.InitiatePayment(c.Request.Context(), &rpc.InitiatePaymentRequest{
		TripId: req.TripID,
		Email:  req.Email,
		Amount: req.Amount,
	})

	if err != nil {
		logger.Error().Err(err).Str("trip_id", req.TripID).Msg("Failed to initiate processing of payment request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not initiate payment"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandlePaymentCallback(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	var provider types.PaymentProvider
	var payload any

	paystackSignature := c.GetHeader("x-paystack-signature")
	flutterwaveSignature := c.GetHeader("flutterwave-signature")

	rawBody, err := c.GetRawData()
	if err != nil {
		logger.Error().Err(err).Msg("Error parsing raw data from payment webhook payload")
		c.Status(http.StatusBadRequest)
		return
	}

	// Verify webhook signature
	if paystackSignature != "" {
		provider = types.ProviderPaystack

		h := hmac.New(sha512.New, []byte(h.cfg.Env.PaystackSecretKey))
		h.Write(rawBody)
		hash := hex.EncodeToString(h.Sum(nil))

		if hash != paystackSignature {
			logger.Error().Msg("Invalid paystack signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req contracts.PaystackWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Error parsing Paystack webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		payload = req
	} else if flutterwaveSignature != "" {
		provider = types.ProviderFlutterwave

		h := hmac.New(sha256.New, []byte(h.cfg.Env.FlutterwaveSecretHash))
		h.Write(rawBody)
		hash := base64.StdEncoding.EncodeToString(h.Sum(nil))

		if hash != flutterwaveSignature {
			logger.Error().Msg("Invalid flutterwave signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req contracts.FlutterwaveWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error().Err(err).Msg("Error parsing Flutterwave webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		payload = req
	}

	if payload == nil {
		logger.Error().Msg("No payload received")
		c.Status(http.StatusBadRequest)
		return
	}

	// Publish webhook event to payment service
	paymentServiceData, err := json.Marshal(messaging.PaymentQueuePayload{
		Provider: provider,
		Data:     payload,
	})
	if err != nil {
		logger.Error().Err(err).Msg("Failed to marshal payment queue payload")
		c.Status(http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Queue.PublishMessage(
		c.Request.Context(),
		messaging.ServicesExchange,
		messaging.PaymentEventWebhookReceived,
		messaging.AmqpMessage{Data: paymentServiceData},
	); err != nil {
		logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.PaymentEventWebhookReceived)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}
