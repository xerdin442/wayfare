package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
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

type TripUpdateData struct {
	DriverID     string
	NewStatus    types.TripStatus
	Rating       int64
	RiderComment string
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

func (r *TripRepository) StoreRideFares(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userID string) error {
	userIDHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return fmt.Errorf("Invalid user ID: %v", err)
	}

	docs := make([]*models.RideFareModel, 0, len(rideFares))

	for _, fare := range rideFares {
		docs = append(docs, &models.RideFareModel{
			ID:               bson.NewObjectID(),
			UserID:           userIDHex,
			CarPackage:       types.CarPackage(fare.PackageSlug),
			BasePrice:        fare.BasePrice,
			TotalPriceInKobo: fare.TotalPriceInKobo,
			ExpiresAt:        time.Now().Add(15 * time.Minute), // Documents are dropped after 15mins
			Route:            route,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		})
	}

	if _, err := r.rideFareColl.InsertMany(ctx, docs); err != nil {
		return fmt.Errorf("Failed to insert ride_fare documents: %v", err)
	}

	return nil
}

func (r *TripRepository) GetPricingPerRegion(ctx context.Context, pickupCoords orb.Point) ([]*models.PricingModel, error) {
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

	var region models.RegionModel
	err := r.regionColl.FindOne(ctx, filter).Decode(&region)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, fmt.Errorf("Wayfare is not available in this location")
	}

	// Get available pricing categories for the region
	cursor, err := r.pricingColl.Find(ctx, bson.M{"region_id": region.ID})
	if err != nil {
		return nil, fmt.Errorf("Error fetching pricing categories: %v", err)
	}

	var pricingModels []*models.PricingModel
	if err := cursor.All(ctx, &pricingModels); err != nil {
		return nil, fmt.Errorf("Error parsing pricing model documents: %v", err)
	}
	if len(pricingModels) == 0 {
		return nil, fmt.Errorf("No pricing categories found for this region")
	}

	return pricingModels, nil
}

func (r *TripRepository) GetTripByID(ctx context.Context, tripId string) (*models.TripModel, error) {
	tripIDHex, err := bson.ObjectIDFromHex(tripId)
	if err != nil {
		return nil, fmt.Errorf("Invalid trip ID: %v", err)
	}

	var trip models.TripModel
	err = r.tripColl.FindOne(ctx, bson.M{"_id": tripIDHex}).Decode(&trip)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Trip not found")
		}
		return nil, fmt.Errorf("Error fetching trip: %v", err)
	}

	return &trip, nil
}

func (r *TripRepository) CreateTrip(ctx context.Context, fareID, userID string) (*models.TripModel, error) {
	userIDHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, fmt.Errorf("Invalid user ID: %v", err)
	}

	fareIDHex, err := bson.ObjectIDFromHex(fareID)
	if err != nil {
		return nil, fmt.Errorf("Invalid ride fare ID: %v", err)
	}

	var rideFare models.RideFareModel
	err = r.rideFareColl.FindOne(ctx, bson.M{
		"_id":     fareIDHex,
		"user_id": userIDHex,
	}).Decode(&rideFare)

	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Invalid or expired ride fare")
		}
		return nil, fmt.Errorf("Error fetching ride fare: %v", err)
	}

	trip := &models.TripModel{
		ID:     bson.NewObjectID(),
		UserID: userIDHex,
		Status: types.TripStatusSearching,
		Fare: models.RideFareSummary{
			CarPackage:       rideFare.CarPackage,
			BasePrice:        rideFare.BasePrice,
			TotalPriceInKobo: rideFare.TotalPriceInKobo,
		},
		Route:     rideFare.Route,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if _, err = r.tripColl.InsertOne(ctx, trip); err != nil {
		return nil, fmt.Errorf("Failed to insert trip document: %v", err)
	}

	return trip, nil
}

func (r *TripRepository) UpdateTrip(ctx context.Context, tripID string, data *TripUpdateData) (*models.TripModel, error) {
	tripIDHex, err := bson.ObjectIDFromHex(tripID)
	if err != nil {
		return nil, fmt.Errorf("Invalid trip ID: %v", err)
	}

	updateData := bson.M{}

	if data.DriverID != "" {
		driverIDHex, err := bson.ObjectIDFromHex(data.DriverID)
		if err != nil {
			return nil, fmt.Errorf("Invalid driver ID: %v", err)
		}

		updateData["driver_id"] = driverIDHex
	}

	if data.NewStatus != "" {
		updateData["status"] = data.NewStatus
	}

	if data.Rating != 0 {
		updateData["rating"] = data.Rating
	}

	if data.RiderComment != "" {
		updateData["rider_comment"] = data.RiderComment
	}

	updateData["updated_at"] = time.Now()

	update := bson.M{
		"$set": updateData,
	}

	if _, err := r.tripColl.UpdateOne(ctx, bson.M{"_id": tripIDHex}, update); err != nil {
		return nil, fmt.Errorf("Failed to update trip document: %v", err)
	}

	return r.GetTripByID(ctx, tripID)
}

func (r *TripRepository) UpdateDriverRatings(ctx context.Context) error {
	oneYearAgo := time.Now().AddDate(-1, 0, 0)

	// Calculate driver rating using Bayesian Average: ((C * m) + S) / (C + N)
	// C = Confidence value
	// m = Global mean
	// S = Sum of ratings
	// N = Number of ratings

	const GlobalMean = 3.0
	const ConfidenceValue = 7.0

	pipeline := mongo.Pipeline{
		// Match trips that have a valid rating
		{{Key: "$match", Value: bson.M{"rating": bson.M{"$gt": 0}}}},

		// Group ratings within the past year and calculate Bayesian values
		{{Key: "$group", Value: bson.M{
			"_id": "$driver_id",
			"numOfTrips": bson.M{
				"$sum": bson.M{
					"$cond": bson.A{bson.M{"$gt": bson.A{"$created_at", oneYearAgo}}, 1, 0},
				},
			},
			"sumOfRatings": bson.M{
				"$sum": bson.M{
					"$cond": bson.A{bson.M{"$gt": bson.A{"$created_at", oneYearAgo}}, "$rating", 0},
				},
			},
			"lifetimeAvg": bson.M{"$avg": "$rating"},
		}}},

		// Project final ratings using Bayesian formula
		{{Key: "$project", Value: bson.M{
			"lifetime_rating_avg": "$lifetimeAvg",
			"current_rating": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$numOfTrips", 0}},
					0.0,
					bson.M{
						"$divide": bson.A{
							bson.M{"$add": bson.A{bson.M{"$multiply": bson.A{ConfidenceValue, GlobalMean}}, "$sumOfRatings"}},
							bson.M{"$add": bson.A{ConfidenceValue, "$numOfTrips"}},
						},
					},
				},
			},
			"updated_at": time.Now(),
		}}},

		// Update driver profiles with calculated ratings
		{{Key: "$merge", Value: bson.M{
			"into":           "drivers",
			"on":             "_id",
			"whenMatched":    "merge",
			"whenNotMatched": "discard",
		}}},
	}

	cursor, err := r.tripColl.Aggregate(ctx, pipeline)
	if err != nil {
		return fmt.Errorf("Failed to aggregate and update ratings: %v", err)
	}
	return cursor.Close(ctx)
}
