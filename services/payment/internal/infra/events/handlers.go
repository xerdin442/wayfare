package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
	repo "github.com/xerdin442/wayfare/services/payment/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
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

func (h *PaymentEventsHandler) HandleWebhook(ctx context.Context, p messaging.AmqpDeliveryPayload) error {
	var msg messaging.AmqpMessage
	if err := json.Unmarshal(p.Body, &msg); err != nil {
		return fmt.Errorf("Failed to unmarshal message from %s event: %v", p.RoutingKey, err)
	}

	var payload messaging.PaymentQueuePayload
	if err := json.Unmarshal(msg.Data, &payload); err != nil {
		return fmt.Errorf("Failed to unmarshal payload from %s event: %v", p.RoutingKey, err)
	}

	return nil
}
