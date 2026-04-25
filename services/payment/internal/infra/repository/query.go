package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	rpc "github.com/xerdin442/wayfare/shared/pkg"
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type PaymentRepository struct {
	txnColl *mongo.Collection
}

func NewPaymentRepository(db *mongo.Database) *PaymentRepository {
	txnCollection, err := CreateTransactionsCollection(db, "transactions")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create transactions collection")
	}

	return &PaymentRepository{
		txnColl: txnCollection,
	}
}

func (r *PaymentRepository) CreateTransaction(ctx context.Context, details *rpc.InitiatePaymentRequest) (string, error) {
	tripIDHex, err := bson.ObjectIDFromHex(details.TripId)
	if err != nil {
		return "", fmt.Errorf("Invalid trip ID: %v", err)
	}

	txn := &models.TransactionModel{
		ID:        bson.NewObjectID(),
		TripID:    tripIDHex,
		Email:     details.Email,
		Amount:    details.Amount,
		Status:    types.PaymentStatusPending,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}

	_, insertErr := r.txnColl.InsertOne(ctx, txn)
	if insertErr != nil {
		return "", fmt.Errorf("Failed to insert transaction document: %v", insertErr)
	}

	return txn.ID.Hex(), nil
}

func (r *PaymentRepository) UpdateTransaction(ctx context.Context, txnID string, status types.PaymentStatus, provider types.PaymentProvider) error {
	txnIDHex, err := bson.ObjectIDFromHex(txnID)
	if err != nil {
		return fmt.Errorf("Invalid transaction ID: %v", err)
	}

	updateData := bson.M{
		"status":     status,
		"provider":   provider,
		"updated_at": time.Now().UTC(),
	}

	update := bson.M{
		"$set": updateData,
	}

	_, updateErr := r.txnColl.UpdateOne(
		ctx,
		bson.M{"_id": txnIDHex},
		update,
	)
	if updateErr != nil {
		return fmt.Errorf("Failed to update transaction document: %v", updateErr)
	}

	return nil
}
