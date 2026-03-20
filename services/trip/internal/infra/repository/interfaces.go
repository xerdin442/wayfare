package repo

import (
	"context"

	"github.com/xerdin442/wayfare/services/trip/internal/models"
)

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *models.Trip) (*models.Trip, error)
}
