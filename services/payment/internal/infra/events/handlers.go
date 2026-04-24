package events

import (
	"context"

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
	return nil
}
