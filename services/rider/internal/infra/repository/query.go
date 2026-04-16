package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/bcrypt"
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
	riderIDHex, err := bson.ObjectIDFromHex(riderId)
	if err != nil {
		return nil, fmt.Errorf("Invalid rider ID: %v", err)
	}

	var rider models.RiderModel
	err = r.riderColl.FindOne(ctx, bson.M{"_id": riderIDHex}).Decode(&rider)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Rider not found")
		}
		return nil, fmt.Errorf("Error fetching rider: %v", err)
	}

	return &rider, nil
}

func (r *RiderRepository) GetRiderByEmail(ctx context.Context, email string) (*models.RiderModel, error) {
	var rider models.RiderModel
	err := r.riderColl.FindOne(ctx, bson.M{"email": email}).Decode(&rider)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Rider not found")
		}
		return nil, fmt.Errorf("Error fetching rider: %v", err)
	}

	return &rider, nil
}

func (r *RiderRepository) CreateRiderAccount(ctx context.Context, details *rpc.SignupRiderRequest) (*models.RiderModel, error) {
	// Generate password hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(details.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("Failed to hash password: %v", err)
	}

	rider := &models.RiderModel{
		ID:             bson.NewObjectID(),
		Name:           details.Name,
		Email:          details.Email,
		Password:       string(hashedPassword),
		ProfilePicture: details.ProfileImage,
	}

	_, insertErr := r.riderColl.InsertOne(ctx, rider)
	if insertErr != nil {
		return nil, fmt.Errorf("Failed to create rider account: %v", insertErr)
	}

	return rider, nil
}
