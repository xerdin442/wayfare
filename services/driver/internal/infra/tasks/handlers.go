package tasks

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
)

type DriverTasksHandler struct {
	repo *repo.DriverRepository
}

func NewDriverTasksHandler(r *repo.DriverRepository) *DriverTasksHandler {
	return &DriverTasksHandler{
		repo: r,
	}
}

func (h *DriverTasksHandler) ResetDriverBalances(ctx context.Context) error {
	return h.repo.BatchResetBalances(ctx)
}

func (h *DriverTasksHandler) ProcessDriverPayouts(ctx context.Context) error {
	return nil
}
