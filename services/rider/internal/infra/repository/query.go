package repo

import (
	"context"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type RiderRepository struct {
	riderColl *mongo.Collection
}

func NewRiderRepository(db *mongo.Database) *RiderRepository {
	riderCollection, err := CreateRidersCollection(db, "riders")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create riders collection")
	}

	return &RiderRepository{
		riderColl: riderCollection,
	}
}

func (r *RiderRepository) GetRiderByID(ctx context.Context, riderId string) (*models.RiderModel, error) {
	return &models.RiderModel{}, nil
}
