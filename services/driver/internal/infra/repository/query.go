package repo

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type DriverRepository struct {
	driverColl *mongo.Collection
}

func NewDriverRepository(db *mongo.Database) *DriverRepository {
	driverCollection, err := CreateDriversCollection(db, "drivers")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create drivers collection")
	}

	return &DriverRepository{
		driverColl: driverCollection,
	}
}

func (r *DriverRepository) GetDriverByID(ctx context.Context, driverId string) (*DriverModel, error) {
	return &DriverModel{}, nil
}
