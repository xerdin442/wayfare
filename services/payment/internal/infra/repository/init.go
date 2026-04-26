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
		"required": []string{"trip_id", "email", "amount", "status", "created_at", "updated_at"},
		"properties": bson.M{
			"trip_id": bson.M{"bsonType": "objectId"},
			"provider": bson.M{
				"enum":        []string{"paystack", "flutterwave"},
				"description": "must be one of the supported payment providers",
			},
			"email":  bson.M{"bsonType": "string"},
			"amount": bson.M{"bsonType": "long", "minimum": 1},
			"status": bson.M{
				"enum":        []string{"pending", "success", "failed", "refunded"},
				"description": "must be a valid payment status value",
			},
			"created_at": bson.M{"bsonType": "date"},
			"updated_at": bson.M{"bsonType": "date"},
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

	_, err := collection.Indexes().CreateOne(ctx, tripIndex)
	if err != nil {
		return nil, err
	}

	return collection, nil
}
