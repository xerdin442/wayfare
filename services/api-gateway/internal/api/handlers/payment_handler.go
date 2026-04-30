package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
)

var (
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	ErrInvalidWebhookEvent     = errors.New("invalid webhook event")
	ErrEmptyWebhookPayload     = errors.New("empty webhook payload")
)

func (h *RouteHandler) HandlePaymentCallback(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandlePaymentCallback")
	defer span.End()

	logger := log.Ctx(ctx)

	userID := c.MustGet("user_id").(string)

	var provider types.PaymentProvider
	var payload any

	paystackSignature := c.GetHeader("x-paystack-signature")
	flutterwaveSignature := c.GetHeader("verif-hash")

	rawBody, err := c.GetRawData()
	if err != nil {
		tracing.HandleError(span, err)
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
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid paystack signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req contracts.PaystackWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Error parsing Paystack webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		if !strings.HasPrefix(req.Event, "charge.") {
			tracing.HandleError(span, ErrInvalidWebhookEvent)
			logger.Warn().Msg("Invalid Paystack webhook event")
			c.Status(http.StatusBadRequest)
			return
		}

		payload = req
	} else if flutterwaveSignature != "" {
		provider = types.ProviderFlutterwave

		if h.cfg.Env.FlutterwaveVerifHash != flutterwaveSignature {
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid flutterwave signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req contracts.FlutterwaveWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Error parsing Flutterwave webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		if !strings.HasPrefix(req.Event, "charge.") {
			tracing.HandleError(span, ErrInvalidWebhookEvent)
			logger.Warn().Msg("Invalid Flutterwave webhook event")
			c.Status(http.StatusBadRequest)
			return
		}

		payload = req
	}

	if payload == nil {
		tracing.HandleError(span, ErrEmptyWebhookPayload)
		logger.Error().Msg("No payload received")
		c.Status(http.StatusBadRequest)
		return
	}

	// Publish webhook event to payment service
	paymentServiceData, err := json.Marshal(messaging.CheckoutPaymentPayload{
		RiderID:  userID,
		Provider: provider,
		Data:     payload,
	})
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Failed to marshal payment queue payload")
		c.Status(http.StatusInternalServerError)
		return
	}

	if err := h.cfg.Queue.PublishMessage(
		ctx,
		messaging.ServicesExchange,
		messaging.PaymentEventWebhookReceived,
		messaging.AmqpMessage{Data: paymentServiceData},
	); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msgf("Failed to publish %s event", messaging.PaymentEventWebhookReceived)
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}
