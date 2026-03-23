package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type TripRepository struct {
	regionColl   *mongo.Collection
	pricingColl  *mongo.Collection
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
			{Key: "route.pickup.coordinates", Value: "2dsphere"},
			{Key: "route.destination.coordinates", Value: "2dsphere"},
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

func (r *TripRepository) StoreRideFares(ctx context.Context, rideFares []*rpc.RideFare, route RouteDetails, userID string) error {
	userIDHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("Invalid user ID: %v", err)
	}

	docs := make([]*RideFareModel, len(rideFares))

	for _, fare := range rideFares {
		docs = append(docs, &RideFareModel{
			ID:               bson.NewObjectID(),
			UserID:           userIDHex,
			CarPackage:       CarPackage(fare.PackageSlug),
			BasePrice:        fare.BasePrice,
			TotalPriceInKobo: fare.TotalPriceInKobo,
			ExpiresAt:        time.Now().UTC().Add(15 * time.Minute), // Documents are dropped after 15mins
			Route:            route,
			CreatedAt:        time.Now().UTC(),
			UpdatedAt:        time.Now().UTC(),
		})
	}

	_, insertErr := r.rideFareColl.InsertMany(ctx, docs)
	if insertErr != nil {
		return fmt.Errorf("Failed to insert ride fares in database: %v", err)
	}

	return nil
}

func (r *TripRepository) GetPricingPerRegion(ctx context.Context, pickupCoords []float64) ([]*PricingModel, error) {
	// Filter region based on pickup coordinates
	filter := bson.M{
		"boundary": bson.M{
			"$geoIntersects": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": pickupCoords,
				},
			},
		},
	}

	var region RegionModel
	err := r.regionColl.FindOne(ctx, filter).Decode(&region)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("Wayfare is not available in this location")
	}

	// Get available pricing categories for the region
	cursor, err := r.pricingColl.Find(ctx, bson.M{"region_id": region.ID})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("No pricing categories found. Invalid region value")
	}

	var pricingModels []*PricingModel
	if err := cursor.All(ctx, &pricingModels); err != nil {
		return nil, fmt.Errorf("Error parsing pricing model documents: %v", err)
	}

	return pricingModels, nil
}
