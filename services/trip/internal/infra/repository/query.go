package repo

import (
	"context"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TripRepository struct {
	rideFareColl *mongo.Collection
	tripColl     *mongo.Collection
}

func NewTripRepository(db *mongo.Database) *TripRepository {
	ctx := context.Background()

	// Create collections in the database
	rideFareCollection := db.Collection("ride_fares")
	tripCollection := db.Collection("trips")

	// Create expiration index for ride fares
	fareExpirationIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	_, rideIndexErr := rideFareCollection.Indexes().CreateOne(ctx, fareExpirationIndex)
	if rideIndexErr != nil {
		log.Fatal().Err(rideIndexErr).Msg("Failed to create expiration index in ride_fares collection")
		return nil
	}

	// Create 2dsphere index for trip routes
	routeIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "fare.route.pickup", Value: "2dsphere"},
			{Key: "fare.route.destination", Value: "2dsphere"},
		},
	}
	_, tripIndexErr := tripCollection.Indexes().CreateOne(ctx, routeIndex)
	if tripIndexErr != nil {
		log.Fatal().Err(tripIndexErr).Msg("Failed to create location index in trips collection")
		return nil
	}

	return &TripRepository{
		rideFareColl: rideFareCollection,
		tripColl:     tripCollection,
	}
}

func (r *TripRepository) CreateTrip(ctx context.Context) error {
	return nil
}
