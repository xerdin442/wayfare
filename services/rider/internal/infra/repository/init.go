package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func CreateRidersCollection(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"name", "email", "password", "profile_picture"},
		"properties": bson.M{
			"name":            bson.M{"bsonType": "string"},
			"email":           bson.M{"bsonType": "string"},
			"password":        bson.M{"bsonType": "string"},
			"profile_picture": bson.M{"bsonType": "string"},
		},
	}

	// Set schema validator
	validator := bson.M{"$jsonSchema": jsonSchema}
	opts := options.CreateCollection().SetValidator(validator)

	if err := db.CreateCollection(ctx, name, opts); err != nil {
		return nil, err
	}

	collection := db.Collection(name)

	emailIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "email", Value: 1}},
	}

	if _, err := collection.Indexes().CreateOne(ctx, emailIndex); err != nil {
		return nil, err
	}

	return collection, nil
}
