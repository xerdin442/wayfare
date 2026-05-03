package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type PaymentRepository struct {
	txnColl *mongo.Collection
}

type CreateTransactionData struct {
	TripID   string
	DriverID string
	Amount   int64
	Type     types.TransactionType
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

func (r *PaymentRepository) GetTransactionByID(ctx context.Context, txnID string) (*models.TransactionModel, error) {
	txnIDHex, err := bson.ObjectIDFromHex(txnID)
	if err != nil {
		return nil, fmt.Errorf("Invalid transaction ID: %v", err)
	}

	var transaction models.TransactionModel
	err = r.txnColl.FindOne(ctx, bson.M{"_id": txnIDHex}).Decode(&transaction)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Transaction not found")
		}
		return nil, fmt.Errorf("Error fetching transaction: %v", err)
	}

	return &transaction, nil
}

func (r *PaymentRepository) GetTransactionByTripID(ctx context.Context, tripID string) (*models.TransactionModel, error) {
	tripIDHex, err := bson.ObjectIDFromHex(tripID)
	if err != nil {
		return nil, fmt.Errorf("Invalid trip ID: %v", err)
	}

	var transaction models.TransactionModel
	err = r.txnColl.FindOne(ctx, bson.M{"trip_id": tripIDHex}).Decode(&transaction)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("Transaction not found")
		}
		return nil, fmt.Errorf("Error fetching transaction: %v", err)
	}

	return &transaction, nil
}

func (r *PaymentRepository) CreateTransaction(ctx context.Context, data *CreateTransactionData) (string, error) {
	txn := &models.TransactionModel{
		ID:        bson.NewObjectID(),
		Amount:    data.Amount,
		Type:      data.Type,
		Status:    types.PaymentStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if data.TripID != "" {
		tripIDHex, err := bson.ObjectIDFromHex(data.TripID)
		if err != nil {
			return "", fmt.Errorf("Invalid trip ID: %v", err)
		}

		if data.Type != types.TransactionCheckout {
			return "", fmt.Errorf("Invalid payload type for checkout transaction: %v", data.Type)
		}

		txn.TripID = tripIDHex
	}

	if data.DriverID != "" {
		driverIDHex, err := bson.ObjectIDFromHex(data.DriverID)
		if err != nil {
			return "", fmt.Errorf("Invalid driver ID: %v", err)
		}

		if data.Type != types.TransactionPayout {
			return "", fmt.Errorf("Invalid payload type for payout transaction: %v", data.Type)
		}

		txn.DriverID = driverIDHex
	}

	if _, err := r.txnColl.InsertOne(ctx, txn); err != nil {
		return "", fmt.Errorf("Failed to insert transaction document: %v", err)
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
		"updated_at": time.Now().UTC(),
	}

	if provider != "" {
		updateData["provider"] = provider
	}

	update := bson.M{
		"$set": updateData,
	}

	if _, err = r.txnColl.UpdateOne(ctx, bson.M{"_id": txnIDHex}, update); err != nil {
		return fmt.Errorf("Failed to update transaction document: %v", err)
	}

	return nil
}
