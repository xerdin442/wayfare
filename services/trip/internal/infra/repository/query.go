package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

type TripRepository struct {
	collection *mongo.Collection
}

func NewTripRepository(db *mongo.Database) *TripRepository {
	return &TripRepository{
		collection: db.Collection("trips"),
	}
}

func (r *TripRepository) CreateTrip(ctx context.Context) error {
	return nil
}
