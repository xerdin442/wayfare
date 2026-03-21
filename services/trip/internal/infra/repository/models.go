package repo

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type RouteModel struct {
	Location GeoJSON `bson:"location"`
	Duration float64 `bson:"duration"`
	Distance float64 `bson:"distance"`
}

type RouteFareModel struct {
	ID               primitive.ObjectID `bson:"_id,omitempty"`
	PackageSlug      string             `bson:"package_slug"`
	BasePrice        float64            `bson:"base_price"`
	TotalPriceInKobo float64            `bson:"total_price_in_kobo"`
	ExpiresAt        time.Time          `bson:"expires_at"`
	Route            RouteModel         `bson:"route"`
	CreatedAt        time.Time          `bson:"created_at"`
	UpdatedAt        time.Time          `bson:"updated_at"`
}

type TripModel struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	DriverID  primitive.ObjectID `bson:"driver_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	Status    string             `bson:"status"`
	Fare      RouteFareModel     `bson:"fare"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type GeoJSON struct {
	Type        string    `bson:"type"`        // Must be "Point"
	Coordinates []float64 `bson:"coordinates"` // [longitude, latitude]
}
