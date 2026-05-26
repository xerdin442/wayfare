package handlers

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrInvalidWebhookSignature = errors.New("invalid webhook signature")
	ErrInvalidWebhookEvent     = errors.New("invalid webhook event")
	ErrEmptyWebhookPayload     = errors.New("empty webhook payload")
)

func (h *RouteHandler) HandleInitiateCheckout(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandleInitiateCheckout")
	defer span.End()

	logger := log.Ctx(ctx)

	userId := c.MustGet("user_id").(string)
	tripId := c.Param("id")

	txnType := types.TransactionReturns
	if tripId != "" {
		txnType = types.TransactionRideFare
	}

	var req contracts.InitiateCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error parsing initiate checkout request")
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	idempotencyKey := fmt.Sprintf("lock:payment:%s", userId)

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
		c.JSON(http.StatusConflict, gin.H{"message": "Payment request is already being processed"})
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
	checkoutResponse, err := h.cfg.Clients.Payment.InitiateCheckout(ctx, &pb.InitiateCheckoutRequest{
		TripId:       tripId,
		UserId:       userId,
		Email:        req.Email,
		TxnType:      string(txnType),
		TripRating:   req.TripRating,
		RiderComment: req.RiderComment,
		DriverTip:    req.DriverTip,
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
				c.JSON(http.StatusNotFound, gin.H{"message": st.Message()})
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

	c.JSON(http.StatusOK, contracts.APIResponse{
		Data: gin.H{
			"checkoutUrl": checkoutResponse.CheckoutUrl,
		},
	})
}

func (h *RouteHandler) HandlePaymentCallback(c *gin.Context) {
	// Start tracer
	ctx, span := h.cfg.Tracer.Start(c.Request.Context(), "HandlePaymentCallback")
	defer span.End()

	logger := log.Ctx(ctx)

	paystackSignature := c.GetHeader("x-paystack-signature")
	flutterwaveSignature := c.GetHeader("verif-hash")

	rawBody, err := c.GetRawData()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error parsing raw data from payment webhook payload")
		c.Status(http.StatusBadRequest)
		return
	}

	var queuePayload messaging.PaymentWebhookPayload

	// Verify webhook signature
	if paystackSignature != "" {
		h := hmac.New(sha512.New, []byte(h.cfg.Env.PaystackSecretKey))
		h.Write(rawBody)
		hash := hex.EncodeToString(h.Sum(nil))

		if hash != paystackSignature {
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid paystack signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req *contracts.PaystackWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Error parsing paystack webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		if !strings.HasPrefix(req.Event, "charge.") && !strings.HasPrefix(req.Event, "transfer.") {
			tracing.HandleError(span, ErrInvalidWebhookEvent)
			logger.Warn().Msgf("Invalid paystack webhook event: %s", req.Event)
			c.Status(http.StatusBadRequest)
			return
		}

		queuePayload.Provider = types.ProviderPaystack
		queuePayload.PaystackWebhook = req
	} else if flutterwaveSignature != "" {
		if h.cfg.Env.FlutterwaveVerifHash != flutterwaveSignature {
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid flutterwave signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req *contracts.FlutterwaveWebhookPayload
		if err := c.ShouldBindJSON(&req); err != nil {
			tracing.HandleError(span, err)
			logger.Error().Err(err).Msg("Error parsing flutterwave webhook payload")
			c.Status(http.StatusBadRequest)
			return
		}

		if !strings.HasPrefix(req.Event, "charge.") {
			tracing.HandleError(span, ErrInvalidWebhookEvent)
			logger.Warn().Msg("Invalid flutterwave webhook event")
			c.Status(http.StatusBadRequest)
			return
		}

		queuePayload.Provider = types.ProviderFlutterwave
		queuePayload.FlutterwaveWebhook = req
	} else {
		tracing.HandleError(span, ErrEmptyWebhookPayload)
		logger.Error().Msg("No webhook payload received")
		c.Status(http.StatusBadRequest)
		return
	}

	// Publish webhook event to payment service
	paymentServiceData, err := json.Marshal(queuePayload)
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
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Status(http.StatusOK)
}
