package tasks

import (
	"context"

	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
)

type TripTasksHandler struct {
	repo *repo.TripRepository
}

func NewTripTasksHandler(r *repo.TripRepository) *TripTasksHandler {
	return &TripTasksHandler{
		repo: r,
	}
}

func (h *TripTasksHandler) HandleDriverRatingsUpdate(ctx context.Context) error {
	return h.repo.UpdateDriverRatings(ctx)
}
