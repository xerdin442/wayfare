package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
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
	userRole := c.MustGet("user_role").(types.UserRole)
	tripId := c.Param("id")

	txnType := types.TransactionReturns
	requiredRole := types.RoleDriver
	lockTarget := "returns"
	if tripId != "" {
		txnType = types.TransactionRideFare
		requiredRole = types.RoleRider
		lockTarget = tripId
	}

	if userRole != requiredRole {
		c.JSON(http.StatusForbidden, gin.H{"message": fmt.Sprintf("Only %ss can make this payment", requiredRole)})
		return
	}

	var req contracts.InitiateCheckoutRequest
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

	if txnType == types.TransactionRideFare && req.TripRating == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Validation failed",
			"errors":  gin.H{"tripRating": "tripRating is required"},
		})
		return
	}

	// Rating, comment and tip only apply to ride fares
	if txnType == types.TransactionReturns {
		req.TripRating, req.RiderComment, req.DriverTip = 0, "", 0
	}

	idempotencyKey := fmt.Sprintf("lock:payment:%s:%s", userId, lockTarget)
	acquired, err := h.cfg.Cache.SetNX(ctx, idempotencyKey, "locked", 2*time.Minute).Result()
	if err != nil {
		tracing.HandleError(span, err)
		logger.Error().Err(err).Msg("Error setting idempotency lock")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "An error occurred while processing payment"})
		return
	}
	if !acquired {
		c.JSON(http.StatusConflict, gin.H{"message": "Payment request is already being processed"})
		return
	}
	defer func() {
		if err := h.cfg.Cache.Del(context.WithoutCancel(ctx), idempotencyKey).Err(); err != nil {
			logger.Error().Err(err).Msg("Error removing idempotency lock")
		}
	}()

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

		st, ok := status.FromError(err)
		if ok {
			switch st.Code() {
			case codes.NotFound:
				c.JSON(http.StatusNotFound, gin.H{"message": st.Message()})
			case codes.FailedPrecondition:
				c.JSON(http.StatusConflict, gin.H{"message": st.Message()})
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
		mac := hmac.New(sha512.New, []byte(h.cfg.Env.PaystackSecretKey))
		mac.Write(rawBody)
		expectedSignature := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expectedSignature), []byte(paystackSignature)) {
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid paystack signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req *contracts.PaystackWebhookPayload
		if err := json.Unmarshal(rawBody, &req); err != nil {
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
		if subtle.ConstantTimeCompare([]byte(h.cfg.Env.FlutterwaveVerifHash), []byte(flutterwaveSignature)) != 1 {
			tracing.HandleError(span, ErrInvalidWebhookSignature)
			logger.Error().Msg("Invalid flutterwave signature")
			c.Status(http.StatusBadRequest)
			return
		}

		var req *contracts.FlutterwaveWebhookPayload
		if err := json.Unmarshal(rawBody, &req); err != nil {
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
