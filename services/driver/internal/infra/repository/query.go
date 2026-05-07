package repo

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type DriverRepository struct {
	driverColl *mongo.Collection
}

type DriverUpdateData struct {
	Status                  types.DriverStatus
	TripCountUpdate         bool
	SplitAmount             int64
	BalanceUpdate           bool
	PendingReturnsUpdate    bool
	OutstandingReturnsReset bool
}

func NewDriverRepository(db *mongo.Database) *DriverRepository {
	driverCollection, err := models.CreateDriversCollection(db, "drivers")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create drivers collection")
	}

	return &DriverRepository{
		driverColl: driverCollection,
	}
}

func (r *DriverRepository) GetDriverByID(ctx context.Context, driverId string) (*models.DriverModel, error) {
	driverIdHex, err := bson.ObjectIDFromHex(driverId)
	if err != nil {
		log.Error().Err(err).Str("id", driverId).Msg("Invalid driver ID")
		return nil, err
	}

	var driver models.DriverModel
	err = r.driverColl.FindOne(ctx, bson.M{"_id": driverIdHex}).Decode(&driver)
	if err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrDocumentNotFound
		}
		return nil, err
	}

	return &driver, nil
}

func (r *DriverRepository) GetDriverByEmail(ctx context.Context, email string) (*models.DriverModel, error) {
	var driver models.DriverModel
	err := r.driverColl.FindOne(ctx, bson.M{"email": email}).Decode(&driver)
	if err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database query error")
		return nil, err
	}

	return &driver, nil
}

func (r *DriverRepository) CreateDriverAccount(ctx context.Context, details *pb.SignupDriverRequest) (*models.DriverModel, error) {
	// Generate password hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(details.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return nil, err
	}

	driver := &models.DriverModel{
		ID:                    bson.NewObjectID(),
		Name:                  details.Name,
		Email:                 details.Email,
		Password:              string(hashedPassword),
		ProfilePicture:        details.ProfileImage,
		CarPackage:            types.CarPackage(details.CarPackage),
		CarPlate:              details.CarPlate,
		CurrentRating:         0.0,
		TotalCompletedTrips:   0,
		LifetimeRatingAvg:     0.0,
		AvailableBalance:      0,
		PendingPayout:         0,
		PendingReturns:        0,
		OutstandingReturns:    0,
		TransferRecipientCode: details.TransferRecipientCode,
		Tier:                  types.TierBronze,
		Status:                types.DriverStatusOffline,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if _, err := r.driverColl.InsertOne(ctx, driver); err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database insert error")
		return nil, err
	}

	return driver, nil
}

func (r *DriverRepository) UpdateDriverDetails(ctx context.Context, driverId string, data *DriverUpdateData) error {
	driverIdHex, err := bson.ObjectIDFromHex(driverId)
	if err != nil {
		log.Error().Err(err).Str("id", driverId).Msg("Invalid driver ID")
		return err
	}

	setFields := bson.M{
		"updated_at": time.Now(),
	}

	incFields := bson.M{}

	if data.TripCountUpdate {
		incFields["total_completed_trips"] = 1
	}
	if data.BalanceUpdate {
		incFields["available_balance"] = data.SplitAmount
	}
	if data.PendingReturnsUpdate {
		incFields["pending_returns"] = data.SplitAmount
	}
	if data.OutstandingReturnsReset {
		setFields["outstanding_returns"] = 0
	}
	if data.Status != "" {
		setFields["status"] = data.Status
	}

	updateData := bson.M{
		"$set": setFields,
	}

	if len(incFields) > 0 {
		updateData["$inc"] = incFields
	}

	if _, err := r.driverColl.UpdateOne(ctx, bson.M{"_id": driverIdHex}, updateData); err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database update error")
		return err
	}

	return nil
}

func (r *DriverRepository) BatchResetBalances(ctx context.Context) error {
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.D{
			// Transfer earnings to payout if they exceed 2000 naira
			{Key: "pending_payout", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$gt", Value: bson.A{"$available_balance", 200000}}},
				bson.D{{Key: "$add", Value: bson.A{"$pending_payout", "$available_balance"}}},
				"$pending_payout",
			}}}},
			// Reset balance if earnings were transferred to payout
			{Key: "available_balance", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$gt", Value: bson.A{"$available_balance", 200000}}},
				0,
				"$available_balance",
			}}}},
			// Update outstanding returns if there are any pending returns
			{Key: "outstanding_returns", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$gt", Value: bson.A{"$pending_returns", 0}}},
				bson.D{{Key: "$add", Value: bson.A{"$outstanding_returns", "$pending_returns"}}},
				"$outstanding_returns",
			}}}},
			// Reset pending returns
			{Key: "pending_returns", Value: 0},
			{Key: "updated_at", Value: time.Now()},
		}}},
	}

	if _, err := r.driverColl.UpdateMany(ctx, bson.M{}, pipeline); err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database update error")
		return err
	}

	return nil
}

func (r *DriverRepository) GetDriversForPayout(ctx context.Context) ([]*models.DriverModel, error) {
	cursor, err := r.driverColl.Find(ctx, bson.M{"pending_payout": bson.M{"$gt": 0}})
	if err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database query error")
		return nil, err
	}
	defer cursor.Close(ctx)

	var drivers []*models.DriverModel
	if err := cursor.All(ctx, &drivers); err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database cursor error")
		return nil, err
	}

	return drivers, nil
}

func (r *DriverRepository) ResetPendingPayout(ctx context.Context, recipientCode string) error {
	filter := bson.M{"transfer_recipient_code": recipientCode}
	update := bson.M{
		"$set": bson.M{
			"pending_payout": 0,
			"updated_at":     time.Now(),
		},
	}

	if _, err := r.driverColl.UpdateOne(ctx, filter, update); err != nil {
		log.Error().Err(err).Str("collection", "drivers").Msg("Database update error")
		return err
	}

	return nil
}
