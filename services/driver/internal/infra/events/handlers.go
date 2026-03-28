package events

import (
	"context"

	"github.com/redis/go-redis/v9"
	repo "github.com/xerdin442/wayfare/services/driver/internal/infra/repository"
	"github.com/xerdin442/wayfare/shared/messaging"
)

type DriverEventsHandler struct {
	repo  *repo.DriverRepository
	cache *redis.Client
	bus   messaging.MessageBus
}

func NewDriverEventsHandler(r *repo.DriverRepository, b messaging.MessageBus, c *redis.Client) *DriverEventsHandler {
	return &DriverEventsHandler{
		repo:  r,
		cache: c,
		bus:   b,
	}
}

func (h *DriverEventsHandler) findAvailableDrivers() ([]string, error)

func (h *DriverEventsHandler) HandleTripCreated(ctx context.Context, body []byte) error

func (h *DriverEventsHandler) HandleDriverNotInterested(ctx context.Context, body []byte) error

func (h *DriverEventsHandler) HandleDriverNotAvailable(ctx context.Context, body []byte) error
