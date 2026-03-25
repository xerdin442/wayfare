package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type TripRepository struct {
	regionColl   *mongo.Collection
	pricingColl  *mongo.Collection
	rideFareColl *mongo.Collection
	tripColl     *mongo.Collection
}

func NewTripRepository(db *mongo.Database) *TripRepository {
	regionCollection, err := CreateRegionsCollection(db, "regions")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create regions collection")
	}

	pricingCollection, err := CreatePricingColelction(db, "pricing")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create pricing collection")
	}

	rideFareCollection, err := CreateRideFaresColelction(db, "ride_fares")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create ride_fares collection")
	}

	tripCollection, err := CreateTripsColelction(db, "trips")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create trips collection")
	}

	return &TripRepository{
		regionColl:   regionCollection,
		pricingColl:  pricingCollection,
		rideFareColl: rideFareCollection,
		tripColl:     tripCollection,
	}
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
			CarPackage:       types.CarPackage(fare.PackageSlug),
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
		return fmt.Errorf("Failed to insert ride fares documents: %v", err)
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

func (r *TripRepository) CreateTrip(ctx context.Context, fareID, userID string) (string, error) {
	userIDHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return "", fmt.Errorf("Invalid user ID: %v", err)
	}

	fareIDHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return "", fmt.Errorf("Invalid ride fare ID: %v", err)
	}

	cursor, err := r.rideFareColl.Find(ctx, bson.M{
		"_id":     fareIDHex,
		"user_id": userIDHex,
	})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", fmt.Errorf("Invalid or expired ride fare")
	}

	var rideFare RideFareModel
	if err := cursor.All(ctx, &rideFare); err != nil {
		return "", fmt.Errorf("Error parsing ridefare model document: %v", err)
	}

	result, insertErr := r.tripColl.InsertOne(ctx, TripModel{
		ID:     bson.NewObjectID(),
		UserID: userIDHex,
		Status: StatusSearching,
		Fare: RideFareSummary{
			CarPackage:       rideFare.CarPackage,
			BasePrice:        rideFare.BasePrice,
			TotalPriceInKobo: rideFare.TotalPriceInKobo,
		},
		Route:     rideFare.Route,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	if insertErr != nil {
		return "", fmt.Errorf("Failed to insert trip document: %v", err)
	}

	tripID, ok := result.InsertedID.(bson.ObjectID)
	if !ok {
		return "", fmt.Errorf("Mongo error: Invalid ID type from inserted document")
	}

	return tripID.Hex(), nil
}
