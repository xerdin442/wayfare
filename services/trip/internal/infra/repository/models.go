package repo

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type CarPackage string

const (
	PackageLuxury CarPackage = "luxury"
	PackageSedan  CarPackage = "sedan"
	PackageSUV    CarPackage = "suv"
)

type GeoPoint struct {
	Type        string    `bson:"type"`
	Coordinates []float64 `bson:"coordinates"`
}

type GeoPolygon struct {
	Type        string        `bson:"type"`
	Coordinates [][][]float64 `bson:"coordinates"`
}

type RouteDetails struct {
	Pickup      GeoPoint `bson:"pickup"`
	Destination GeoPoint `bson:"destination"`
	Duration    float64  `bson:"duration"`
	Distance    float64  `bson:"distance"`
	Polyline    string   `bson:"polyline,omitempty"`
}

type RideFareSummary struct {
	CarPackage       CarPackage `bson:"car_package"`
	BasePrice        int64      `bson:"base_price"`
	TotalPriceInKobo int64      `bson:"total_price_in_kobo"`
}

type RegionModel struct {
	ID       bson.ObjectID `bson:"_id,omitempty"`
	Name     string        `bson:"name"`
	Boundary GeoPolygon    `bson:"boundary"`
}

type PricingModel struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	RegionID      bson.ObjectID `bson:"region_id"`
	CarPackage    CarPackage    `bson:"car_package"`
	BaseFeeKobo   int64         `bson:"base_fee_kobo"`
	PerKmKobo     int64         `bson:"per_km_kobo"`
	PerMinuteKobo int64         `bson:"per_minute_kobo"`
	MinFareKobo   int64         `bson:"min_fare_kobo"`
}

type RideFareModel struct {
	ID               bson.ObjectID `bson:"_id,omitempty"`
	UserID           bson.ObjectID `bson:"user_id"`
	CarPackage       CarPackage    `bson:"car_package"`
	BasePrice        int64         `bson:"base_price"`
	TotalPriceInKobo int64         `bson:"total_price_in_kobo"`
	ExpiresAt        time.Time     `bson:"expires_at"`
	Route            RouteDetails  `bson:"route"`
	CreatedAt        time.Time     `bson:"created_at"`
	UpdatedAt        time.Time     `bson:"updated_at"`
}

type TripModel struct {
	ID        bson.ObjectID   `bson:"_id,omitempty"`
	DriverID  bson.ObjectID   `bson:"driver_id"`
	UserID    bson.ObjectID   `bson:"user_id"`
	Status    string          `bson:"status"`
	Fare      RideFareSummary `bson:"fare"`
	Route     RouteDetails    `bson:"route"`
	CreatedAt time.Time       `bson:"created_at"`
	UpdatedAt time.Time       `bson:"updated_at"`
}
