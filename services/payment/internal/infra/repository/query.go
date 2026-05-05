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
	TripID              string
	DriverRecipientCode string
	Amount              int64
	Type                types.TransactionType
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

	if data.DriverRecipientCode != "" {
		if data.Type != types.TransactionPayout {
			return "", fmt.Errorf("Invalid payload type for payout transaction: %v", data.Type)
		}

		txn.DriverRecipientCode = data.DriverRecipientCode
	}

	if _, err := r.txnColl.InsertOne(ctx, txn); err != nil {
		return "", fmt.Errorf("Failed to insert transaction document: %v", err)
	}

	return txn.ID.Hex(), nil
}

func (r *PaymentRepository) CreateBatchTransactions(ctx context.Context, data []CreateTransactionData) ([]string, error) {
	var txns []*models.TransactionModel
	for _, d := range data {
		txns = append(txns, &models.TransactionModel{
			ID:                  bson.NewObjectID(),
			Amount:              d.Amount,
			Type:                d.Type,
			Status:              types.PaymentStatusPending,
			DriverRecipientCode: d.DriverRecipientCode,
			CreatedAt:           time.Now(),
			UpdatedAt:           time.Now(),
		})
	}

	result, err := r.txnColl.InsertMany(ctx, txns)
	if err != nil {
		return nil, fmt.Errorf("Failed to insert batch transactions: %v", err)
	}

	var ids []string
	for _, id := range result.InsertedIDs {
		ids = append(ids, id.(bson.ObjectID).Hex())
	}

	return ids, nil
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

func (r *PaymentRepository) UpdateBatchTransactions(ctx context.Context, txnIDs []string, status types.PaymentStatus, provider types.PaymentProvider) error {
	var idsHex []bson.ObjectID
	for _, id := range txnIDs {
		hex, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return fmt.Errorf("Invalid transaction ID in batch: %v", err)
		}
		idsHex = append(idsHex, hex)
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

	if _, err := r.txnColl.UpdateMany(ctx, bson.M{"_id": bson.M{"$in": idsHex}}, update); err != nil {
		return fmt.Errorf("Failed to update batch transactions: %v", err)
	}

	return nil
}

func (r *PaymentRepository) GetRecentPayoutTransactions(ctx context.Context) ([]*models.TransactionModel, error) {
	filter := bson.M{
		"type":       types.TransactionPayout,
		"created_at": bson.M{"$gte": time.Now().Add(-5 * time.Hour)},
	}

	cursor, err := r.txnColl.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch recent payout transactions: %v", err)
	}
	defer cursor.Close(ctx)

	var transactions []*models.TransactionModel
	if err := cursor.All(ctx, &transactions); err != nil {
		return nil, fmt.Errorf("Failed to decode payout transactions: %v", err)
	}

	return transactions, nil
}
