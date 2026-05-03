package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func CreateTransactionsCollection(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"amount", "status", "type"},
		"properties": bson.M{
			"trip_id":   bson.M{"bsonType": "objectId"},
			"driver_id": bson.M{"bsonType": "objectId"},
			"type": bson.M{
				"enum":        []string{"checkout", "payout"},
				"description": "must be a valid transaction type",
			},
			"provider": bson.M{
				"enum":        []string{"paystack", "flutterwave"},
				"description": "must be one of the supported payment providers",
			},
			"amount": bson.M{"bsonType": "long"},
			"status": bson.M{
				"enum":        []string{"pending", "success", "failed", "reversed", "aborted"},
				"description": "must be a valid payment status value",
			},
		},
	}

	// Set schema validator
	validator := bson.M{"$jsonSchema": jsonSchema}
	opts := options.CreateCollection().SetValidator(validator)

	if err := db.CreateCollection(ctx, name, opts); err != nil {
		return nil, err
	}

	collection := db.Collection(name)

	// Create search index
	tripIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "trip_id", Value: 1}},
	}

	if _, err := collection.Indexes().CreateOne(ctx, tripIndex); err != nil {
		return nil, err
	}

	return collection, nil
}
