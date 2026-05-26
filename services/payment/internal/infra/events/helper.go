package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xerdin442/wayfare/shared/contracts"
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/types"
)

func (h *PaymentEventsHandler) sendTransactionStatus(ctx context.Context, userID string, status types.PaymentStatus) error {
	var event messaging.AmqpEvent
	if status == types.PaymentStatusSuccess {
		event = messaging.PaymentEventSuccess
	} else {
		event = messaging.PaymentEventFailed
	}

	gatewayData, err := json.Marshal(contracts.WebsocketMessage{
		Type: string(event),
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

func (h *PaymentEventsHandler) markTripAsCompleted(ctx context.Context, p *messaging.TripUpdateQueuePayload) error {
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

func (h *PaymentEventsHandler) calculateTransactionFee(amount int64, p types.PaymentProvider, t types.TransactionType) float64 {
	switch p {
	case types.ProviderPaystack:
		if t == types.TransactionPayout {
			var fee float64
			switch {
			case amount <= 5000:
				fee = 10
			case amount > 5000 && amount <= 50000:
				fee = 25
			case amount > 50000:
				fee = 50
			}

			if amount >= 10000 {
				fee += 50
			}

			return fee
		}

		fee := float64(amount) * 0.015
		if amount >= 2500 {
			fee += 100
			if fee > 2000 {
				fee = 2000
			}
		}

		return fee
	case types.ProviderFlutterwave:
		charge := float64(amount) * 0.02
		vat := charge * 0.075
		return charge + vat
	}

	return 0
}

func (h *PaymentEventsHandler) checkTransferRetries(ctx context.Context, recipientCode string) error {
	transactions, err := h.repo.GetRecentPayoutTransactions(ctx, recipientCode)
	if err != nil {
		return err
	}

	if len(transactions) >= 2 {
		// Send email
	}

	return nil
}
