package events

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
	"github.com/shopspring/decimal"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/services/payment/internal/secrets"
	"github.com/xerdin442/wayfare/shared/analytics"
	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/tracing"
	"github.com/xerdin442/wayfare/shared/types"
)

type PaymentEventsHandler struct {
	repo       *repo.PaymentRepository
	bus        messaging.MessageBus
	cache      *redis.Client
	env        *secrets.Secrets
	httpClient *http.Client
}

func NewPaymentEventsHandler(r *repo.PaymentRepository, b messaging.MessageBus, c *redis.Client, s *secrets.Secrets) *PaymentEventsHandler {
	return &PaymentEventsHandler{
		repo:       r,
		bus:        b,
		cache:      c,
		env:        s,
		httpClient: tracing.NewHttpClient(),
	}
}

func (h *PaymentEventsHandler) HandleWebhook(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.PaymentWebhookPayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	var tripServicePayload *messaging.TripUpdateQueuePayload
	var tripEvent *models.TripEventModel
	var updatedStatus types.PaymentStatus

	switch payload.Provider {
	case types.ProviderPaystack:
		webhook := payload.PaystackWebhook
		recipientCode := webhook.Data.Recipient.RecipientCode

		var metadata contracts.PaymentMetadata
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

		if transaction.Amount == webhook.Data.Amount {
			switch webhook.Data.Status {
			case string(types.PaymentStatusSuccess):
				updatedStatus = types.PaymentStatusSuccess

				// Reset driver's pending payout after confirmation
				if webhook.Event == "transfer.success" {
					data, err := json.Marshal(messaging.DriverUpdateQueuePayload{
						RecipientCode: recipientCode,
					})
					if err != nil {
						return fmt.Errorf("Failed to marshal driver_update queue payload")
					}

					if err := h.bus.PublishMessage(
						ctx,
						messaging.ServicesExchange,
						messaging.DriverCmdDetailsUpdate,
						messaging.AmqpMessage{Data: data},
					); err != nil {
						return fmt.Errorf("Failed to publish %s event", messaging.DriverCmdDetailsUpdate)
					}
				}
			case string(types.PaymentStatusFailed):
				updatedStatus = types.PaymentStatusFailed
			case string(types.PaymentStatusReversed):
				updatedStatus = types.PaymentStatusReversed
			}

			if strings.HasPrefix(webhook.Event, "transfer.") {
				if err := h.checkTransferRetries(ctx, recipientCode); err != nil {
					log.Error().Err(err).Str("recipient", recipientCode).Msg("Error checking transfer retries")
				}
			}
		} else {
			return fmt.Errorf("Invalid Paystack webhook payload")
		}

		if err := h.repo.UpdateTransaction(
			ctx,
			transaction.ID.Hex(),
			updatedStatus,
			types.ProviderPaystack,
		); err != nil {
			return err
		}

		if strings.HasPrefix(webhook.Event, "charge.") {
			// Send transaction status to rider
			if err := h.sendTransactionStatus(ctx, metadata.UserID, updatedStatus); err != nil {
				log.Warn().Err(err).Str("txn_id", transaction.ID.Hex()).Msg("Failed to send transaction status to rider")
			}

			if updatedStatus == types.PaymentStatusSuccess && metadata.TripID != "" {
				tripServicePayload = &messaging.TripUpdateQueuePayload{
					TripID:         metadata.TripID,
					RiderComment:   metadata.RiderComment,
					Rating:         metadata.TripRating,
					DriverTip:      metadata.DriverTip,
					TransactionFee: webhook.Data.ProcessingFee,
				}
			}
		}

		tripEvent = &models.TripEventModel{
			TransactionRef:  webhook.Data.Reference,
			PaymentProvider: payload.Provider,
			PaymentStatus:   updatedStatus,
			Amount:          decimal.NewFromInt(transaction.Amount / 100),
			TransactionType: transaction.Type,
			TransactionFee:  decimal.NewFromInt(webhook.Data.ProcessingFee / 100),
		}

	case types.ProviderFlutterwave:
		webhook := payload.FlutterwaveWebhook
		metadata := webhook.Data.Meta

		paidAmount := int64(math.Round(webhook.Data.Amount * 100))
		txnFee := int64(math.Round(webhook.Data.ProcessingFee * 100))

		transaction, err := h.repo.GetTransactionByID(ctx, webhook.Data.TxRef)
		if err != nil {
			return err
		}

		// Idempotent processing to skip settled transactions
		if transaction.Status != types.PaymentStatusPending {
			return nil
		}

		if transaction.Amount == paidAmount && webhook.Event == "charge.completed" {
			switch webhook.Data.Status {
			case "successful":
				updatedStatus = types.PaymentStatusSuccess
			case "failed":
				updatedStatus = types.PaymentStatusFailed
			}
		} else {
			return fmt.Errorf("Invalid Flutterwave webhook payload")
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
		if err := h.sendTransactionStatus(ctx, metadata.UserID, updatedStatus); err != nil {
			log.Warn().Err(err).Str("txn_id", transaction.ID.Hex()).Msg("Failed to send transaction status to rider")
		}

		if updatedStatus == types.PaymentStatusSuccess {
			tripServicePayload = &messaging.TripUpdateQueuePayload{
				TripID:         metadata.TripID,
				RiderComment:   metadata.RiderComment,
				Rating:         metadata.TripRating,
				DriverTip:      metadata.DriverTip,
				TransactionFee: txnFee,
			}
		}

		tripEvent = &models.TripEventModel{
			TransactionRef:  webhook.Data.TxRef,
			PaymentProvider: payload.Provider,
			PaymentStatus:   updatedStatus,
			Amount:          decimal.NewFromInt(paidAmount),
			TransactionType: transaction.Type,
			TransactionFee:  decimal.NewFromInt(txnFee),
		}

	default:
		log.Warn().Str("provider", string(payload.Provider)).Msg("Webhook received from unknown payment provider")
		return fmt.Errorf("Unknown payment provider: %s", payload.Provider)
	}

	if tripServicePayload != nil {
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
	existingTxn, err := h.repo.GetTransactionByFilterID(ctx, payload.TripID)
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
		// Create new checkout transaction
		txnID, err = h.repo.CreateTransaction(ctx, &repo.CreateTransactionData{
			TripID: payload.TripID,
			Amount: payload.Amount,
			Type:   types.TransactionRideFare,
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
	tripServicePayload := &messaging.TripUpdateQueuePayload{
		TripID:      payload.TripID,
		CashPayment: true,
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
		TransactionType: types.TransactionRideFare,
	}
	if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
		return err
	}

	return nil
}

func (h *PaymentEventsHandler) HandleDriverPayout(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payoutPayload messaging.DriverPayoutPayload
	if err := json.Unmarshal(msg.Data, &payoutPayload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	// Check payment gateway status
	gatewayStatusKey := "gateway:paystack:status"
	n, err := h.cache.Exists(ctx, gatewayStatusKey).Result()
	if err != nil {
		return fmt.Errorf("Error fetching gateway status from cache")
	}

	if n > 0 {
		log.Error().Msg("Failed to process driver payout. Paystack API is currently unavailable")
		return fmt.Errorf("Payment gateway is currently unavailable")
	}

	// Create payout transactions
	var txnData []repo.CreateTransactionData
	for _, d := range payoutPayload.Drivers {
		txnFee := h.calculateTransferFee(d.PendingPayout)
		txnData = append(txnData, repo.CreateTransactionData{
			DriverRecipientCode: d.TransferRecipientCode,
			Amount:              d.PendingPayout - txnFee,
			Type:                types.TransactionPayout,
			TransferFee:         txnFee,
		})
	}

	txnIDs, err := h.repo.CreateBatchTransactions(ctx, txnData)
	if err != nil {
		return err
	}

	// Configure bulk transfer payload
	var transfers []*contracts.TransferDetails
	for i, d := range payoutPayload.Drivers {
		txnFee := h.calculateTransferFee(d.PendingPayout)
		transfers = append(transfers, &contracts.TransferDetails{
			Amount:    d.PendingPayout - txnFee,
			Recipient: d.TransferRecipientCode,
			Reference: txnIDs[i],
			Reason:    "WAYFARE INC. - Driver Payout",
		})
	}

	transferPayload, err := json.Marshal(contracts.BulkTransferPayload{
		Currency:  "NGN",
		Source:    "balance",
		Transfers: transfers,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal bulk transfer payload. %s", err.Error())
	}

	// Configure request details
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		h.env.PaystackApiUrl+"/transfer/bulk",
		bytes.NewBuffer(transferPayload),
	)
	if err != nil {
		return fmt.Errorf("Error configuring bulk transfer request. %s", err.Error())
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.env.PaystackSecretKey)

	response, err := h.httpClient.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Error sending bulk transfer request to Paystack API")
		return fmt.Errorf("Error sending bulk transfer request")
	}
	defer response.Body.Close()

	if response.StatusCode >= 500 {
		// Mark payment gateway as unavailable
		if err := h.cache.Set(ctx, gatewayStatusKey, "unavailable", 30*time.Minute).Err(); err != nil {
			log.Error().Err(err).Msg("Error setting gateway status")
		}

		// Update all newly created payout transactions
		if err := h.repo.UpdateBatchTransactions(ctx, txnIDs, types.PaymentStatusAborted, types.ProviderPaystack); err != nil {
			return err
		}

		// Update analytics
		for _, t := range transfers {
			tripEvent := &models.TripEventModel{
				TransactionRef:  t.Reference,
				PaymentProvider: types.ProviderPaystack,
				PaymentStatus:   types.PaymentStatusAborted,
				Amount:          decimal.NewFromInt(t.Amount / 100),
				TransactionType: types.TransactionPayout,
			}
			if err := analytics.SendEvent(ctx, h.bus, tripEvent); err != nil {
				return err
			}
		}

		return fmt.Errorf("Payment gateway is currently unavailable")
	}

	// 15-sec wait to handle queue retries and Paystack API rate limits
	time.Sleep(15 * time.Second)

	return nil
}
