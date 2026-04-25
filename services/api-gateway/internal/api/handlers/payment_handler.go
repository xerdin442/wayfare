package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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

	idempotencyKey := fmt.Sprintf("lock:payment:%s", req.TripID)

	// Check if request is still being processed
	n, err := h.cfg.Cache.Exists(c.Request.Context(), idempotencyKey).Result()
	if err != nil {
		logger.Error().Err(err).Msg("Error fetching idempotency lock from cache")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	if n > 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Request is already being processed"})
		return
	}

	// Set idempotency lock in cache
	if err := h.cfg.Cache.Set(c.Request.Context(), idempotencyKey, "locked", 2*time.Minute).Err(); err != nil {
		logger.Error().Err(err).Msg("Error setting idempotency lock")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	response, err := h.cfg.Clients.Payment.InitiatePayment(c.Request.Context(), &rpc.InitiatePaymentRequest{
		TripId: req.TripID,
		Email:  req.Email,
		Amount: req.Amount,
	})

	if err != nil {
		logger.Error().Err(err).Str("trip_id", req.TripID).Msg("Failed to initiate processing of payment request")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error occurred during payment processing"})
		return
	}

	c.JSON(http.StatusCreated, contracts.APIResponse{Data: response})
}

func (h *RouteHandler) HandlePaymentCallback(c *gin.Context) {
	logger := log.Ctx(c.Request.Context())

	var provider types.PaymentProvider
	var payload any

	paystackSignature := c.GetHeader("x-paystack-signature")
	flutterwaveSignature := c.GetHeader("verif-hash")

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

		if !strings.HasPrefix(req.Event, "charge.") {
			logger.Warn().Msg("Invalid Paystack webhook event")
			c.Status(http.StatusBadRequest)
			return
		}

		payload = req
	} else if flutterwaveSignature != "" {
		provider = types.ProviderFlutterwave

		if h.cfg.Env.FlutterwaveVerifHash != flutterwaveSignature {
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

		if !strings.HasPrefix(req.Event, "charge.") {
			logger.Warn().Msg("Invalid Flutterwave webhook event")
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
