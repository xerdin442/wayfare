package repo

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	pb "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/util"
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
		log.Error().Err(err).Str("id", riderId).Msg("Invalid rider ID")
		return nil, err
	}

	var rider models.RiderModel
	err = r.riderColl.FindOne(ctx, bson.M{"_id": riderIDHex}).Decode(&rider)
	if err != nil {
		log.Error().Err(err).Str("collection", "riders").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrDocumentNotFound
		}
		return nil, err
	}

	return &rider, nil
}

func (r *RiderRepository) GetRiderByEmail(ctx context.Context, email string) (*models.RiderModel, error) {
	var rider models.RiderModel
	err := r.riderColl.FindOne(ctx, bson.M{"email": email}).Decode(&rider)
	if err != nil {
		log.Error().Err(err).Str("collection", "riders").Msg("Database query error")
		return nil, err
	}

	return &rider, nil
}

func (r *RiderRepository) CreateRiderAccount(ctx context.Context, details *pb.SignupRiderRequest) (*models.RiderModel, error) {
	// Generate password hash
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(details.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error().Err(err).Msg("Failed to hash password")
		return nil, err
	}

	rider := &models.RiderModel{
		ID:             bson.NewObjectID(),
		Name:           details.Name,
		Email:          details.Email,
		Password:       string(hashedPassword),
		ProfilePicture: details.ProfileImage,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err = r.riderColl.InsertOne(ctx, rider)
	if err != nil {
		log.Error().Err(err).Str("collection", "riders").Msg("Database insert error")
		return nil, err
	}

	return rider, nil
}
