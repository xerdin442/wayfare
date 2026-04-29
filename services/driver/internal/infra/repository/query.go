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
		ID:                  bson.NewObjectID(),
		Name:                details.Name,
		Email:               details.Email,
		Password:            string(hashedPassword),
		ProfilePicture:      details.ProfileImage,
		CarPackage:          types.CarPackage(details.CarPackage),
		CarPlate:            details.CarPlate,
		CurrentRating:       0.0,
		TotalCompletedTrips: 0,
		LifetimeRatingAvg:   0.0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}

	_, insertErr := r.driverColl.InsertOne(ctx, driver)
	if insertErr != nil {
		return nil, fmt.Errorf("Failed to create driver account: %v", insertErr)
	}

	return driver, nil
}
