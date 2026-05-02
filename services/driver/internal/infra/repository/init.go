package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func CreateDriversCollection(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{
			"name", "email", "password", "profile_picture",
			"car_package", "car_plate", "current_rating",
			"total_completed_trips", "lifetime_rating_avg",
			"available_balance", "pending_balance", "outstanding_returns",
		},
		"properties": bson.M{
			"name":     bson.M{"bsonType": "string"},
			"email":    bson.M{"bsonType": "string"},
			"password": bson.M{"bsonType": "string"},
			"car_package": bson.M{
				"enum":        []string{"luxury", "sedan", "suv"},
				"description": "must be one of the approved car packages",
			},
			"profile_picture":       bson.M{"bsonType": "string"},
			"car_plate":             bson.M{"bsonType": "string"},
			"current_rating":        bson.M{"bsonType": "double", "minimum": 0, "maximum": 5},
			"total_completed_trips": bson.M{"bsonType": "int"},
			"lifetime_rating_avg":   bson.M{"bsonType": "double", "minimum": 0, "maximum": 5},
			"available_balance":     bson.M{"bsonType": "long"},
			"pending_balance":       bson.M{"bsonType": "long"},
			"outstanding_returns":   bson.M{"bsonType": "long"},
		},
	}

	// Set schema validator
	validator := bson.M{"$jsonSchema": jsonSchema}
	opts := options.CreateCollection().SetValidator(validator)

	if err := db.CreateCollection(ctx, name, opts); err != nil {
		return nil, err
	}

	collection := db.Collection(name)

	return collection, nil
}
