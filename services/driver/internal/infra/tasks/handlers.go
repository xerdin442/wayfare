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

func (h *DriverTasksHandler) ResetDriverBalance(ctx context.Context) error {
	return nil
}

func (h *DriverTasksHandler) ProcessDriverPayouts(ctx context.Context) error {
	return nil
}
