package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
)

type DriverTasksHandler struct {
	repo  *repo.DriverRepository
	queue messaging.MessageBus
}

func NewDriverTasksHandler(r *repo.DriverRepository, q messaging.MessageBus) *DriverTasksHandler {
	return &DriverTasksHandler{
		repo:  r,
		queue: q,
	}
}

func (h *DriverTasksHandler) ResetDriverBalances(ctx context.Context) error {
	return h.repo.BatchResetBalances(ctx)
}

func (h *DriverTasksHandler) ProcessDriverPayouts(ctx context.Context) error {
	drivers, err := h.repo.GetDriversForPayout(ctx)
	if err != nil {
		return err
	}

	if len(drivers) == 0 {
		return nil
	}

	// Chunk drivers in batches for Paystack bulk transfer
	batchSize := 100
	for i := 0; i < len(drivers); i += batchSize {
		end := min(i+batchSize, len(drivers))

		data, err := json.Marshal(messaging.DriverPayoutPayload{
			Drivers: drivers[i:end],
		})
		if err != nil {
			return fmt.Errorf("Failed to marshal driver_payout queue payload: %v", err)
		}

		if err := h.queue.PublishMessage(
			ctx,
			messaging.ServicesExchange,
			messaging.PaymentCmdDriverPayout,
			messaging.AmqpMessage{Data: data},
		); err != nil {
			return fmt.Errorf("Failed to publish %s event: %v", messaging.PaymentCmdDriverPayout, err)
		}
	}

	return nil
}
