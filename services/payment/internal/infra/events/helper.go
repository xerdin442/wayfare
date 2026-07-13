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

func (h *PaymentEventsHandler) calculateTransferFee(amount int64) int64 {
	var fee int64
	switch {
	case amount <= 500000:
		fee = 1000
	case amount > 500000 && amount <= 5000000:
		fee = 2500
	case amount > 5000000:
		fee = 5000
	}

	if amount >= 1000000 {
		fee += 5000 // stamp duty
	}

	return fee
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
