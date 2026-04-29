package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
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

func (h *PaymentEventsHandler) markTripAsCompleted(ctx context.Context, p messaging.PaymentQueuePayload) error {
	tripServiceData, err := json.Marshal(messaging.TripUpdateQueuePayload{
		TripID:       p.TripID,
		Rating:       p.TripRating,
		RiderComment: p.RiderComment,
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

	var payload messaging.PaymentQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	switch payload.Provider {
	case types.ProviderPaystack:
		data := payload.Data.(contracts.PaystackWebhookPayload)

		transaction, err := h.repo.GetTransactionByID(ctx, data.Data.Reference)
		if err != nil {
			log.Warn().Err(err).Msg("Invalid transaction reference from Paystack webhook")
			return err
		}

		// Idempotent processing to skip settled transactions
		if transaction.Status != types.PaymentStatusPending {
			return nil
		}

		var updatedStatus types.PaymentStatus
		if transaction.Amount == data.Data.Amount {
			if data.Event == "charge.success" && data.Data.Status == "success" {
				updatedStatus = types.PaymentStatusSuccess
			} else if data.Event == "charge.failed" && data.Data.Status == "failed" {
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

		// Mark trip as completed after successful payment
		if data.Event == "charge.success" && data.Data.Status == "success" {
			if err := h.markTripAsCompleted(ctx, payload); err != nil {
				return err
			}
		}
	case types.ProviderFlutterwave:
		data := payload.Data.(contracts.FlutterwaveWebhookPayload)

		transaction, err := h.repo.GetTransactionByID(ctx, data.Data.TxRef)
		if err != nil {
			return err
		}

		// Idempotent processing to skip settled transactions
		if transaction.Status != types.PaymentStatusPending {
			return nil
		}

		var updatedStatus types.PaymentStatus
		if transaction.Amount == data.Data.Amount && data.Event == "charge.completed" {
			switch data.Data.Status {
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

		// Mark trip as completed after successful payment
		if data.Event == "charge.completed" && data.Data.Status == "successful" {
			if err := h.markTripAsCompleted(ctx, payload); err != nil {
				return err
			}
		}
	default:
		log.Warn().Str("provider", string(payload.Provider)).Msg("Webhook received from unknown payment provider")
		return fmt.Errorf("Unknown payment provider: %s", payload.Provider)
	}

	return nil
}

func (h *PaymentEventsHandler) HandleCashPayment(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.PaymentQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

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
		if err := h.repo.UpdateTransaction(ctx, existingTxn.ID.Hex(), types.PaymentStatusSuccess, ""); err != nil {
			return err
		}
	} else {
		// Create new transaction
		txnID, err := h.repo.CreateTransaction(ctx, payload.TripID, payload.Amount)
		if err != nil {
			return err
		}

		// Set transaction status
		if err := h.repo.UpdateTransaction(ctx, txnID, types.PaymentStatusSuccess, ""); err != nil {
			return err
		}
	}

	// Mark trip as completed
	if err := h.markTripAsCompleted(ctx, payload); err != nil {
		return err
	}

	return nil
}
