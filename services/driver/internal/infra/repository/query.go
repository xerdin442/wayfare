package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
)

type DriverRepository struct {
	driverColl *mongo.Collection
}

type DriverUpdateData struct {
	TripCountUpdate      bool
	SplitAmount          int64
	BalanceUpdate        bool
	PendingReturnsUpdate bool
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

func (r *DriverRepository) GetDriverByID(ctx context.Context, driverId string) (*models.DriverModel, error) {
	driverIDHex, err := bson.ObjectIDFromHex(driverId)
	if err != nil {
		return nil, fmt.Errorf("Invalid driver ID: %v", err)
	}

	var driver models.DriverModel
	err = r.driverColl.FindOne(ctx, bson.M{"_id": driverIDHex}).Decode(&driver)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Driver not found")
		}
		return nil, fmt.Errorf("Error fetching driver: %v", err)
	}

	return &driver, nil
}

func (r *DriverRepository) GetDriverByEmail(ctx context.Context, email string) (*models.DriverModel, error) {
	var driver models.DriverModel
	err := r.driverColl.FindOne(ctx, bson.M{"email": email}).Decode(&driver)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Driver not found")
		}
		return nil, fmt.Errorf("Error fetching driver: %v", err)
	}

	return &driver, nil
}

func (r *DriverRepository) CreateDriverAccount(ctx context.Context, details *pb.SignupDriverRequest) (*models.DriverModel, error) {
	// Generate password hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(details.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("Failed to hash password: %v", err)
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
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	if _, err := r.driverColl.InsertOne(ctx, driver); err != nil {
		return nil, fmt.Errorf("Failed to create driver account: %v", err)
	}

	return driver, nil
}

func (r *DriverRepository) UpdateDriverDetails(ctx context.Context, driverID string, data *DriverUpdateData) error {
	driverIDHex, err := bson.ObjectIDFromHex(driverID)
	if err != nil {
		return fmt.Errorf("Invalid driver ID: %v", err)
	}

	updateData := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	if data.TripCountUpdate {
		updateData["$inc"] = bson.M{
			"total_completed_trips": 1,
		}
	}
	if data.BalanceUpdate {
		updateData["$inc"] = bson.M{
			"available_balance": data.SplitAmount,
		}
	}
	if data.PendingReturnsUpdate {
		updateData["$inc"] = bson.M{
			"pending_returns": data.SplitAmount,
		}
	}

	if _, err := r.driverColl.UpdateOne(ctx, bson.M{"_id": driverIDHex}, updateData); err != nil {
		return fmt.Errorf("Failed to update driver details: %v", err)
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

	_, err := r.driverColl.UpdateMany(ctx, bson.M{}, pipeline)
	return err
}
