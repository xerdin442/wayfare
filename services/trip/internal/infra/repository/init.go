package repo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var routeSchema = bson.M{
	"bsonType": "object",
	"required": []string{"pickup", "destination", "distance", "duration"},
	"properties": bson.M{
		"pickup": bson.M{
			"bsonType": "object",
			"required": []string{"type", "coordinates"},
			"properties": bson.M{
				"type": bson.M{"enum": []string{"Point"}},
				"coordinates": bson.M{
					"bsonType": "array",
					"minItems": 2,
					"maxItems": 2,
					"items":    bson.M{"bsonType": "double"},
				},
			},
		},
		"destination": bson.M{
			"bsonType": "object",
			"required": []string{"type", "coordinates"},
			"properties": bson.M{
				"type": bson.M{"enum": []string{"Point"}},
				"coordinates": bson.M{
					"bsonType": "array",
					"minItems": 2,
					"maxItems": 2,
					"items":    bson.M{"bsonType": "double"},
				},
			},
		},
		"distance": bson.M{"bsonType": "double", "minimum": 0},
		"duration": bson.M{"bsonType": "double", "minimum": 0},
		"polyline": bson.M{"bsonType": "string"},
	},
}

var carPackageSchema = bson.M{
	"enum":        []string{"luxury", "sedan", "suv"},
	"description": "must be one of the approved car packages",
}

var priceSchema = bson.M{
	"bsonType": "long",
	"minimum":  1,
}

func CreateRegionsCollection(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"name", "boundary"},
		"properties": bson.M{
			"name": bson.M{"bsonType": "string"},
			"boundary": bson.M{
				"bsonType": "object",
				"required": []string{"type", "coordinates"},
				"properties": bson.M{
					"type": bson.M{
						"enum": []string{"Polygon"},
					},
					"coordinates": bson.M{
						"bsonType": "array",
						"items": bson.M{
							"bsonType": "array",
							"minItems": 5,
							"items": bson.M{
								"bsonType": "array",
								"minItems": 2,
								"maxItems": 2,
								"items":    bson.M{"bsonType": "double"},
							},
						},
					},
				},
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

	// Create 2dsphere index on boundary field for geospatial queries
	spatialIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "boundary", Value: "2dsphere"}},
	}

	_, err := collection.Indexes().CreateOne(ctx, spatialIndex)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func CreatePricingColelction(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"region_id", "car_package", "base_fee_kobo", "per_km_kobo", "per_minute_kobo", "min_fare_kobo"},
		"properties": bson.M{
			"region_id":       bson.M{"bsonType": "objectId"},
			"car_package":     carPackageSchema,
			"base_fee_kobo":   priceSchema,
			"per_km_kobo":     priceSchema,
			"per_minute_kobo": priceSchema,
			"min_fare_kobo":   priceSchema,
		},
	}

	// Set schema validator
	validator := bson.M{"$jsonSchema": jsonSchema}
	opts := options.CreateCollection().SetValidator(validator)

	if err := db.CreateCollection(ctx, name, opts); err != nil {
		return nil, err
	}

	collection := db.Collection(name)

	// Create region index for faster qeuries
	regionIndex := mongo.IndexModel{
		Keys: bson.D{{Key: "region_id", Value: 1}},
	}

	_, err := collection.Indexes().CreateOne(ctx, regionIndex)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func CreateRideFaresColelction(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"user_id", "car_package", "total_price_in_kobo", "expires_at", "route"},
		"properties": bson.M{
			"user_id":             bson.M{"bsonType": "objectId"},
			"car_package":         carPackageSchema,
			"base_price":          priceSchema,
			"total_price_in_kobo": priceSchema,
			"expires_at":          bson.M{"bsonType": "date"},
			"route":               routeSchema,
		},
	}

	// Set schema validator
	validator := bson.M{"$jsonSchema": jsonSchema}
	opts := options.CreateCollection().SetValidator(validator)

	if err := db.CreateCollection(ctx, name, opts); err != nil {
		return nil, err
	}

	collection := db.Collection(name)

	// Create expiration index
	fareExpirationIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "expires_at", Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(0),
	}
	_, err := collection.Indexes().CreateOne(ctx, fareExpirationIndex)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func CreateTripsColelction(db *mongo.Database, name string) (*mongo.Collection, error) {
	ctx := context.Background()

	jsonSchema := bson.M{
		"bsonType": "object",
		"required": []string{"user_id", "route", "status", "fare"},
		"properties": bson.M{
			"user_id":   bson.M{"bsonType": "objectId"},
			"driver_id": bson.M{"bsonType": "objectId"},
			"route":     routeSchema,
			"status": bson.M{
				"enum":        []string{"searching", "aborted", "matched", "active", "completed", "cancelled"},
				"description": "must be a valid trip status value",
			},
			"fare": bson.M{
				"bsonType": "object",
				"required": []string{"car_package", "base_price", "total_price_in_kobo"},
				"properties": bson.M{
					"car_package":         carPackageSchema,
					"base_price":          priceSchema,
					"total_price_in_kobo": priceSchema,
				},
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

	// Create 2dsphere index for spatial queries on trip routes
	routeIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "route.pickup", Value: "2dsphere"},
			{Key: "route.destination", Value: "2dsphere"},
		},
	}

	// Create search index
	searchIndex := mongo.IndexModel{
		Keys: bson.D{
			{Key: "user_id", Value: 1},
			{Key: "driver_id", Value: 1},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, []mongo.IndexModel{routeIndex, searchIndex})
	if err != nil {
		return nil, err
	}

	return collection, nil
}
