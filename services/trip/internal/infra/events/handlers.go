package events

import (
	repo "github.com/xerdin442/wayfare/services/trip/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
)

type TripEventsHandler struct {
	repo *repo.TripRepository
	bus  messaging.MessageBus
}

func NewTripEventsHandler(r *repo.TripRepository, b messaging.MessageBus) *TripEventsHandler {
	return &TripEventsHandler{
		repo: r,
		bus:  b,
	}
}
