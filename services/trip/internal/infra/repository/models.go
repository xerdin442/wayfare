package repo

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type GeoJSON struct {
	Type        string    `bson:"type"`        // Must be "Point"
	Coordinates []float64 `bson:"coordinates"` // [longitude, latitude]
}

type RouteModel struct {
	Pickup      GeoJSON `bson:"pickup"`
	Destination GeoJSON `bson:"destination"`
	Duration    float64 `bson:"duration"`
	Distance    float64 `bson:"distance"`
	Polyline    string  `bson:"polyline,omitempty"`
}

type RideFareModel struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	PackageSlug      string        `bson:"package_slug"`
	BasePrice        float64       `bson:"base_price"`
	TotalPriceInKobo int64         `bson:"total_price_in_kobo"`
	ExpiresAt        time.Time     `bson:"expires_at"`
	Route            RouteModel    `bson:"route"`
	CreatedAt        time.Time     `bson:"created_at"`
	UpdatedAt        time.Time     `bson:"updated_at"`
}

type TripModel struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	DriverID  bson.ObjectID `bson:"driver_id"`
	UserID    bson.ObjectID `bson:"user_id"`
	Status    string        `bson:"status"`
	Fare      RideFareModel `bson:"fare"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}
