package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/types"
)

type PaymentEventsHandler struct {
	repo  *repo.PaymentRepository
	bus   messaging.MessageBus
	cache *redis.Client
}

func NewPaymentEventsHandler(r *repo.PaymentRepository, b messaging.MessageBus, c *redis.Client) *PaymentEventsHandler {
	return &PaymentEventsHandler{
		repo:  r,
		bus:   b,
		cache: c,
	}
}

func (h *PaymentEventsHandler) sendTransactionStatus(ctx context.Context, userID string, status types.PaymentStatus) error {
	var event messaging.AmqpEvent
	if status == types.PaymentStatusSuccess {
		event = messaging.PaymentEventSuccess
	} else {
		event = messaging.PaymentEventFailed
	}

	gatewayData, err := json.Marshal(contracts.WebsocketMessage{
		Type: event,
	})
	if err != nil {
		return fmt.Errorf("Could not parse event queue payload: %v", err)
	}

	if err := h.bus.PublishMessage(
		ctx,
		messaging.GatewayExchange,
		messaging.AmqpEvent(fmt.Sprintf("user.%s", userID)),
		messaging.AmqpMessage{Data: gatewayData},
	); err != nil {
		return fmt.Errorf("Failed to publish %s event: %v", event, err)
	}

	return nil
}

func (h *PaymentEventsHandler) markTripAsCompleted(ctx context.Context, p messaging.TripUpdateQueuePayload) error {
	tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
		TripID:       p.TripID,
		Rating:       p.Rating,
		RiderComment: p.RiderComment,
		DriverTip:    p.DriverTip,
		CashPayment:  p.CashPayment,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal trip_update queue payload")
	}

	if err := h.bus.PublishMessage(
		ctx,
		messaging.ServicesExchange,
		messaging.TripCmdCompleted,
		messaging.AmqpMessage{Data: tripServiceData},
	); err != nil {
		return fmt.Errorf("Failed to publish %s event", messaging.TripCmdCompleted)
	}

	return nil
}

func (h *PaymentEventsHandler) HandleWebhook(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.CheckoutPaymentPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	var tripServicePayload messaging.TripUpdateQueuePayload
	var tripEvent *models.TripEventModel
	var updatedStatus types.PaymentStatus

	switch payload.Provider {
	case types.ProviderPaystack:
		webhook := payload.PaystackWebhook

		var metadata types.PaymentMetadata
		if err := json.Unmarshal([]byte(webhook.Data.Metadata), &metadata); err != nil {
			return fmt.Errorf("Failed to unmarshal Paystack webhook metadata")
		}

		transaction, err := h.repo.GetTransactionByID(ctx, webhook.Data.Reference)
		if err != nil {
			log.Warn().Err(err).Msg("Invalid transaction reference from Paystack webhook")
			return err
		}

		// Idempotent processing to skip settled transactions
		if transaction.Status != types.PaymentStatusPending {
			return nil
		}

		updatedStatus = types.PaymentStatusAborted
		if transaction.Amount == webhook.Data.Amount/100 {
			if webhook.Event == "charge.success" && webhook.Data.Status == "success" {
				updatedStatus = types.PaymentStatusSuccess
			} else if webhook.Event == "charge.failed" && webhook.Data.Status == "failed" {
				updatedStatus = types.PaymentStatusFailed
			}
		} else {
			return fmt.Errorf("Invalid webhook payload for checkout transaction")
		}

		if err := h.repo.UpdateTransaction(
			ctx,
			transaction.ID.Hex(),
			updatedStatus,
			types.ProviderPaystack,
		); err != nil {
			return err
		}

		// Send transaction status to rider
		if err := h.sendTransactionStatus(ctx, payload.RiderID, updatedStatus); err != nil {
			log.Warn().Err(err).Str("txn_id", transaction.ID.Hex()).Msg("Failed to send transaction status to rider")
		}

		if updatedStatus == types.PaymentStatusSuccess {
			tripServicePayload = messaging.TripUpdateQueuePayload{
				TripID:       metadata.TripID,
				RiderComment: metadata.RiderComment,
				Rating:       metadata.TripRating,
				DriverTip:    metadata.DriverTip,
			}
		}

		tripEvent = &models.TripEventModel{
			TransactionRef:  webhook.Data.Reference,
			PaymentProvider: payload.Provider,
			PaymentStatus:   updatedStatus,
			Amount:          decimal.NewFromInt(webhook.Data.Amount / 100),
			TransactionType: types.TransactionCheckout,
		}

	case types.ProviderFlutterwave:
		webhook := payload.FlutterwaveWebhook
		metadata := webhook.Data.Meta

		transaction, err := h.repo.GetTransactionByID(ctx, webhook.Data.TxRef)
		if err != nil {
			return err
		}

		// Idempotent processing to skip settled transactions
		if transaction.Status != types.PaymentStatusPending {
			return nil
		}

		updatedStatus = types.PaymentStatusAborted
		if transaction.Amount == webhook.Data.Amount && webhook.Event == "charge.completed" {
			switch webhook.Data.Status {
			case "successful":
				updatedStatus = types.PaymentStatusSuccess
			case "failed":
				updatedStatus = types.PaymentStatusFailed
			}
		} else {
			return fmt.Errorf("Invalid webhook payload for checkout transaction")
		}

		if err := h.repo.UpdateTransaction(
			ctx,
			transaction.ID.Hex(),
			updatedStatus,
			types.ProviderFlutterwave,
		); err != nil {
			return err
		}

		// Send transaction status to rider
		if err := h.sendTransactionStatus(ctx, payload.RiderID, updatedStatus); err != nil {
			log.Warn().Err(err).Str("txn_id", transaction.ID.Hex()).Msg("Failed to send transaction status to rider")
		}

		if updatedStatus == types.PaymentStatusSuccess {
			tripServicePayload = messaging.TripUpdateQueuePayload{
				TripID:       metadata.TripID,
				RiderComment: metadata.RiderComment,
				Rating:       metadata.TripRating,
				DriverTip:    metadata.DriverTip,
			}
		}

		tripEvent = &models.TripEventModel{
			TransactionRef:  webhook.Data.TxRef,
			PaymentProvider: payload.Provider,
			PaymentStatus:   updatedStatus,
			Amount:          decimal.NewFromInt(webhook.Data.Amount),
			TransactionType: types.TransactionCheckout,
		}

	default:
		log.Warn().Str("provider", string(payload.Provider)).Msg("Webhook received from unknown payment provider")
		return fmt.Errorf("Unknown payment provider: %s", payload.Provider)
	}

	if updatedStatus == types.PaymentStatusSuccess {
		if err := h.markTripAsCompleted(ctx, tripServicePayload); err != nil {
			return err
		}
	}

	if tripEvent != nil {
		if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
			return err
		}
	}

	return nil
}

func (h *PaymentEventsHandler) HandleCashPayment(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.CashPaymentPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	var txnID string

	// Check for pending transaction
	existingTxn, err := h.repo.GetTransactionByTripID(ctx, payload.TripID)
	if err != nil {
		return err
	}

	// Idempotent processing to skip settled transactions
	if existingTxn.Status != types.PaymentStatusPending {
		return nil
	}

	if existingTxn != nil {
		// Update existing transaction
		txnID = existingTxn.ID.Hex()
		if err := h.repo.UpdateTransaction(ctx, txnID, types.PaymentStatusSuccess, ""); err != nil {
			return err
		}
	} else {
		// Create new transaction
		txnID, err = h.repo.CreateTransaction(ctx, &repo.CreateTransactionData{
			TripID: payload.TripID,
			Amount: payload.Amount,
			Type:   types.TransactionCheckout,
		})
		if err != nil {
			return err
		}

		// Set transaction status
		if err := h.repo.UpdateTransaction(ctx, txnID, types.PaymentStatusSuccess, ""); err != nil {
			return err
		}
	}

	// Send transaction status to rider
	if err := h.sendTransactionStatus(ctx, payload.RiderID, types.PaymentStatusSuccess); err != nil {
		log.Warn().Err(err).Msg("Failed to send transaction status to rider")
	}

	// Mark trip as completed
	tripServicePayload := messaging.TripUpdateQueuePayload{
		TripID:       payload.TripID,
		RiderComment: payload.RiderComment,
		Rating:       payload.TripRating,
		CashPayment:  true,
	}
	if err := h.markTripAsCompleted(ctx, tripServicePayload); err != nil {
		return err
	}

	// Update analytics
	tripEvent := &models.TripEventModel{
		TransactionRef:  txnID,
		PaymentProvider: "none",
		PaymentStatus:   types.PaymentStatusSuccess,
		Amount:          decimal.NewFromInt(payload.Amount / 100),
		TransactionType: types.TransactionCheckout,
	}
	if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
		return err
	}

	return nil
}
