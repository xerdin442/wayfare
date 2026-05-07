package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/shared/models"
	"github.com/xerdin442/wayfare/shared/types"
	"github.com/xerdin442/wayfare/shared/util"
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
	txnCollection, err := models.CreateTransactionsCollection(db, "transactions")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to create transactions collection")
	}

	return &PaymentRepository{
		txnColl: txnCollection,
	}
}

func (r *PaymentRepository) GetTransactionByID(ctx context.Context, txnId string) (*models.TransactionModel, error) {
	txnIdHex, err := bson.ObjectIDFromHex(txnId)
	if err != nil {
		log.Error().Err(err).Str("id", txnId).Msg("Invalid transaction ID")
		return nil, err
	}

	var transaction models.TransactionModel
	err = r.txnColl.FindOne(ctx, bson.M{"_id": txnIdHex}).Decode(&transaction)
	if err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database query error")

		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, util.ErrDocumentNotFound
		}
		return nil, err
	}

	return &transaction, nil
}

func (r *PaymentRepository) GetTransactionByTripID(ctx context.Context, tripId string) (*models.TransactionModel, error) {
	tripIdHex, err := bson.ObjectIDFromHex(tripId)
	if err != nil {
		log.Error().Err(err).Str("id", tripId).Msg("Invalid trip ID")
		return nil, err
	}

	var transaction models.TransactionModel
	err = r.txnColl.FindOne(ctx, bson.M{"trip_id": tripIdHex}).Decode(&transaction)
	if err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database query error")
		return nil, err
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
			log.Error().Err(err).Str("id", data.TripID).Msg("Invalid trip ID")
			return "", err
		}

		if data.Type != types.TransactionCheckout {
			log.Error().Str("type", string(data.Type)).Msg("Invalid payload type for checkout transaction")
			return "", fmt.Errorf("invalid payload type for checkout transaction")
		}

		txn.TripID = tripIDHex
	}

	if data.DriverRecipientCode != "" {
		if data.Type != types.TransactionPayout {
			log.Error().Str("type", string(data.Type)).Msg("Invalid payload type for payout transaction")
			return "", fmt.Errorf("invalid payload type for payout transaction")
		}

		txn.DriverRecipientCode = data.DriverRecipientCode
	}

	if _, err := r.txnColl.InsertOne(ctx, txn); err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database insert error")
		return "", err
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
		log.Error().Err(err).Str("collection", "transactions").Msg("Database insert error")
		return nil, err
	}

	var ids []string
	for _, id := range result.InsertedIDs {
		ids = append(ids, id.(bson.ObjectID).Hex())
	}

	return ids, nil
}

func (r *PaymentRepository) UpdateTransaction(ctx context.Context, txnId string, status types.PaymentStatus, provider types.PaymentProvider) error {
	txnIdHex, err := bson.ObjectIDFromHex(txnId)
	if err != nil {
		log.Error().Err(err).Str("id", txnId).Msg("Invalid transaction ID")
		return err
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

	if _, err = r.txnColl.UpdateOne(ctx, bson.M{"_id": txnIdHex}, update); err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database update error")
		return err
	}

	return nil
}

func (r *PaymentRepository) UpdateBatchTransactions(ctx context.Context, txnIDs []string, status types.PaymentStatus, provider types.PaymentProvider) error {
	var idsHex []bson.ObjectID
	for _, id := range txnIDs {
		hex, err := bson.ObjectIDFromHex(id)
		if err != nil {
			log.Error().Err(err).Str("id", id).Msg("Invalid transaction ID in batch")
			return err
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
		log.Error().Err(err).Str("collection", "transactions").Msg("Database update error")
		return err
	}

	return nil
}

func (r *PaymentRepository) GetRecentPayoutTransactions(ctx context.Context, recipientCode string) ([]*models.TransactionModel, error) {
	filter := bson.M{
		"driver_recipient_code": recipientCode,
		"type":                  types.TransactionPayout,
		"created_at":            bson.M{"$gte": time.Now().Add(-5 * time.Hour)},
	}

	cursor, err := r.txnColl.Find(ctx, filter)
	if err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database query error")
		return nil, err
	}
	defer cursor.Close(ctx)

	var transactions []*models.TransactionModel
	if err := cursor.All(ctx, &transactions); err != nil {
		log.Error().Err(err).Str("collection", "transactions").Msg("Database cursor error")
		return nil, err
	}

	return transactions, nil
}
