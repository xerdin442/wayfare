package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Route struct {
	Location GeoJSON `bson:"location" json:"location"`
	Duration float64 `bson:"duration" json:"duration"`
	Distance float64 `bson:"distance" json:"distance"`
}

type RouteFare struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PackageSlug      string             `bson:"package_slug" json:"packageSlug"`
	BasePrice        float64            `bson:"base_price" json:"basePrice"`
	TotalPriceInKobo float64            `bson:"total_price_in_kobo" json:"totalPriceInKobo"`
	ExpiresAt        time.Time          `bson:"expires_at" json:"expiresAt"`
	Route
}

type Trip struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	DriverID primitive.ObjectID `bson:"driver_id" json:"driverID"`
	UserID   string             `bson:"user_id" json:"userID"`
	Status   string             `bson:"status" json:"status"`
	Fare     RouteFare          `bson:"fare" json:"selectedFare"`
}

type GeoJSON struct {
	Type        string    `bson:"type" json:"type"`               // Must be "Point"
	Coordinates []float64 `bson:"coordinates" json:"coordinates"` // [longitude, latitude]
}
