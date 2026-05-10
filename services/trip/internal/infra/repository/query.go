package repo

import (
	"context"
	"errors"
	"time"

	"github.com/paulmach/orb"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
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

type TripUpdateData struct {
	DriverID     string
	NewStatus    types.TripStatus
	PickupAt     time.Time
	EndedAt      time.Time
	Rating       int64
	RiderComment string
	DriverTip    int64
}

func NewTripRepository(db *mongo.Database) *TripRepository {
	regionCollection, err := models.CreateRegionsCollection(db, "regions")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create regions collection")
	}

	pricingCollection, err := models.CreatePricingColelction(db, "pricing")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create pricing collection")
	}

	rideFareCollection, err := models.CreateRideFaresColelction(db, "ride_fares")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create ride_fares collection")
	}

	tripCollection, err := models.CreateTripsColelction(db, "trips")
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

func (r *TripRepository) StoreRideFares(ctx context.Context, rideFares []*pb.RideFare, route models.RouteDetails, userId, regionId string) error {
	userIdHex, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		log.Error().Err(err).Str("id", userId).Msg("Invalid user ID")
		return err
	}

	regionIdHex, err := bson.ObjectIDFromHex(regionId)
	if err != nil {
		log.Error().Err(err).Str("id", regionId).Msg("Invalid region ID")
		return err
	}

	docs := make([]*models.RideFareModel, 0, len(rideFares))

	for _, fare := range rideFares {
		docs = append(docs, &models.RideFareModel{
			ID:         bson.NewObjectID(),
			UserID:     userIdHex,
			RegionID:   regionIdHex,
			CarPackage: types.CarPackage(fare.PackageSlug),
			Amount:     fare.Amount,
			ExpiresAt:  time.Now().Add(15 * time.Minute), // Documents are dropped after 15mins
			Route:      route,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
	}

	if _, err := r.rideFareColl.InsertMany(ctx, docs); err != nil {
		log.Error().Err(err).Str("collection", "ride_fares").Msg("Database insert error")
		return err
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
	if err != nil {
		log.Error().Err(err).Str("collection", "regions").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrUnsupportedLocation
		}
		return nil, err
	}

	// Get available pricing categories for the region
	cursor, err := r.pricingColl.Find(ctx, bson.M{"region_id": region.ID})
	if err != nil {
		log.Error().Err(err).Str("collection", "pricing").Msg("Database query error")
		return nil, err
	}
	defer cursor.Close(ctx)

	var pricingModels []*models.PricingModel
	if err := cursor.All(ctx, &pricingModels); err != nil {
		log.Error().Err(err).Str("collection", "pricing").Msg("Database cursor error")
		return nil, err
	}
	if len(pricingModels) == 0 {
		return nil, util.ErrUnsupportedLocation
	}

	return pricingModels, nil
}

func (r *TripRepository) GetTripByID(ctx context.Context, tripId string) (*models.TripModel, error) {
	tripIdHex, err := bson.ObjectIDFromHex(tripId)
	if err != nil {
		log.Error().Err(err).Str("id", tripId).Msg("Invalid trip ID")
		return nil, err
	}

	var trip models.TripModel
	err = r.tripColl.FindOne(ctx, bson.M{"_id": tripIdHex}).Decode(&trip)
	if err != nil {
		log.Error().Err(err).Str("collection", "trips").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrDocumentNotFound
		}
		return nil, err
	}

	return &trip, nil
}

func (r *TripRepository) CreateTrip(ctx context.Context, fareId, userId string) (*models.TripModel, error) {
	userIdHex, err := bson.ObjectIDFromHex(userId)
	if err != nil {
		log.Error().Err(err).Str("id", userId).Msg("Invalid user ID")
		return nil, err
	}

	fareIdHex, err := bson.ObjectIDFromHex(fareId)
	if err != nil {
		log.Error().Err(err).Str("id", fareId).Msg("Invalid ride fare ID")
		return nil, err
	}

	var rideFare models.RideFareModel
	err = r.rideFareColl.FindOne(ctx, bson.M{
		"_id":     fareIdHex,
		"user_id": userIdHex,
	}).Decode(&rideFare)

	if err != nil {
		log.Error().Err(err).Str("collection", "ride_fares").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrTripSessionExpired
		}
		return nil, err
	}

	var region models.RegionModel
	err = r.regionColl.FindOne(ctx, bson.M{
		"_id": rideFare.RegionID,
	}).Decode(&region)

	if err != nil {
		log.Error().Err(err).Str("collection", "regions").Msg("Database query error")
		return nil, err
	}

	trip := &models.TripModel{
		ID:         bson.NewObjectID(),
		UserID:     userIdHex,
		Region:     region.Name,
		Status:     types.TripStatusSearching,
		RideFare:   rideFare.Amount,
		CarPackage: rideFare.CarPackage,
		Route:      rideFare.Route,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if _, err = r.tripColl.InsertOne(ctx, trip); err != nil {
		log.Error().Err(err).Str("collection", "trips").Msg("Database insert error")
		return nil, err
	}

	return trip, nil
}

func (r *TripRepository) UpdateTrip(ctx context.Context, tripId string, data *TripUpdateData) (*models.TripModel, error) {
	tripIdHex, err := bson.ObjectIDFromHex(tripId)
	if err != nil {
		log.Error().Err(err).Str("id", tripId).Msg("Invalid trip ID")
		return nil, err
	}

	updateData := bson.M{}

	if data.DriverID != "" {
		driverIdHex, err := bson.ObjectIDFromHex(data.DriverID)
		if err != nil {
			log.Error().Err(err).Str("id", data.DriverID).Msg("Invalid driver ID")
			return nil, err
		}

		updateData["driver_id"] = driverIdHex
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
	if !data.PickupAt.IsZero() {
		updateData["pickup_at"] = data.PickupAt
	}
	if !data.EndedAt.IsZero() {
		updateData["ended_at"] = data.EndedAt
	}
	if data.DriverTip != 0 {
		updateData["driver_tip"] = data.DriverTip
	}

	updateData["updated_at"] = time.Now()

	update := bson.M{
		"$set": updateData,
	}

	if _, err := r.tripColl.UpdateOne(ctx, bson.M{"_id": tripIdHex}, update); err != nil {
		log.Error().Err(err).Str("collection", "trips").Msg("Database update error")
		return nil, err
	}

	return r.GetTripByID(ctx, tripId)
}

func (r *TripRepository) UpdateDriverRatingAndTier(ctx context.Context) error {
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
					GlobalMean,
					bson.M{
						"$divide": bson.A{
							bson.M{"$add": bson.A{bson.M{"$multiply": bson.A{ConfidenceValue, GlobalMean}}, "$sumOfRatings"}},
							bson.M{"$add": bson.A{ConfidenceValue, "$numOfTrips"}},
						},
					},
				},
			},
			"tier": bson.M{
				"$cond": bson.A{
					bson.M{"$gte": bson.A{"$numOfTrips", 1000}},
					types.TierGold,
					bson.M{
						"$cond": bson.A{
							bson.M{"$gte": bson.A{"$numOfTrips", 300}},
							types.TierSilver,
							types.TierBronze,
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
		log.Error().Err(err).Str("collection", "trips").Msg("Database update error")
		return err
	}

	return cursor.Close(ctx)
}

func (r *TripRepository) GetActiveTripRequests(ctx context.Context, pickupCoords orb.Point) (int64, error) {
	filter := bson.M{
		"status": bson.M{
			"$in": []types.TripStatus{
				types.TripStatusSearching,
				types.TripStatusMatched,
				types.TripStatusActive,
				types.TripStatusAwaitingPayment,
			},
		},
		"route.pickup": bson.M{
			"$near": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					"coordinates": pickupCoords,
				},
				"$maxDistance": 5000,
			},
		},
	}

	count, err := r.tripColl.CountDocuments(ctx, filter)
	if err != nil {
		log.Error().Err(err).Str("collection", "trips").Msg("Database count error")
		return 0, err
	}

	return count, nil
}

func (r *TripRepository) GetLastUnratedTrip(ctx context.Context, userID string) (*models.TripModel, error) {
	userIdHex, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		log.Error().Err(err).Str("id", userID).Msg("Invalid user ID")
		return nil, err
	}

	filter := bson.M{
		"user_id": userIdHex,
		"status":  types.TripStatusCompleted,
	}

	var trip models.TripModel
	err = r.tripColl.FindOne(ctx, filter, options.FindOne().SetSort(bson.M{"created_at": -1})).Decode(&trip)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to find last trip")
		return nil, err
	}

	if trip.Rating > 0 {
		return nil, nil
	}

	return &trip, nil
}
