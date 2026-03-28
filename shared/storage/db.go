package storage

import (
	"context"
	"time"

	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func InitializeDatabase(ctx context.Context, uri string) *mongo.Database {
	clientOptions := options.Client().ApplyURI(uri)
	mongoClient, err := mongo.Connect(clientOptions)
	if err != nil {
		log.Fatal().Err(err).Msg("Invalid MongoDB connection URI")
	}
	defer mongoClient.Disconnect(ctx)

	var pingErr error
	for range 3 {
		pingErr = mongoClient.Ping(ctx, nil)
		if pingErr == nil {
			break
		}

		log.Warn().Msg("Waiting for MongoDB...")
		time.Sleep(time.Second * 2)
	}

	if pingErr != nil {
		log.Fatal().Err(pingErr).Msg("Could not connect to MongoDB after 3 attempts. Exiting...")
	}

	return mongoClient.Database("wayfare")
}
