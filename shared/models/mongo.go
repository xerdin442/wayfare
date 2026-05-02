package models

import (
	"time"

	"github.com/paulmach/orb"
	"github.com/xerdin442/wayfare/shared/types"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type GeoPoint struct {
	Type        string    `bson:"type"` // Set as "Point"
	Coordinates orb.Point `bson:"coordinates"`
}

type GeoPolygon struct {
	Type        string      `bson:"type"` // Set as "Polygon"
	Coordinates orb.Polygon `bson:"coordinates"`
}

type RouteDetails struct {
	Pickup      GeoPoint `bson:"pickup"`
	Destination GeoPoint `bson:"destination"`
	Duration    float64  `bson:"duration"`
	Distance    float64  `bson:"distance"`
	Polyline    string   `bson:"polyline,omitempty"`
}

type RideFareSummary struct {
	CarPackage       types.CarPackage `bson:"car_package"`
	BasePrice        int64            `bson:"base_price"`
	TotalPriceInKobo int64            `bson:"total_price_in_kobo"`
}

type RegionModel struct {
	ID        bson.ObjectID `bson:"_id,omitempty"`
	Name      string        `bson:"name"`
	Boundary  GeoPolygon    `bson:"boundary"`
	CreatedAt time.Time     `bson:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at"`
}

type PricingModel struct {
	ID            bson.ObjectID    `bson:"_id,omitempty"`
	RegionID      bson.ObjectID    `bson:"region_id"`
	CarPackage    types.CarPackage `bson:"car_package"`
	BaseFeeKobo   int64            `bson:"base_fee_kobo"`
	PerKmKobo     int64            `bson:"per_km_kobo"`
	PerMinuteKobo int64            `bson:"per_minute_kobo"`
	MinFareKobo   int64            `bson:"min_fare_kobo"`
	CreatedAt     time.Time        `bson:"created_at"`
	UpdatedAt     time.Time        `bson:"updated_at"`
}

type RideFareModel struct {
	ID               bson.ObjectID    `bson:"_id,omitempty"`
	UserID           bson.ObjectID    `bson:"user_id"`
	RegionID         bson.ObjectID    `bson:"region_id"`
	CarPackage       types.CarPackage `bson:"car_package"`
	BasePrice        int64            `bson:"base_price"`
	TotalPriceInKobo int64            `bson:"total_price_in_kobo"`
	ExpiresAt        time.Time        `bson:"expires_at"`
	Route            RouteDetails     `bson:"route"`
	CreatedAt        time.Time        `bson:"created_at"`
	UpdatedAt        time.Time        `bson:"updated_at"`
}

type TripModel struct {
	ID           bson.ObjectID    `bson:"_id,omitempty"`
	DriverID     bson.ObjectID    `bson:"driver_id,omitempty"`
	UserID       bson.ObjectID    `bson:"user_id"`
	Region       string           `bson:"region"`
	Status       types.TripStatus `bson:"status"`
	Fare         RideFareSummary  `bson:"fare"`
	Route        RouteDetails     `bson:"route"`
	PickupAt     time.Time        `bson:"pickup_at,omitempty"`
	EndedAt      time.Time        `bson:"ended_at,omitempty"`
	Rating       int64            `bson:"rating,omitempty"`
	RiderComment string           `bson:"rider_comment,omitempty"`
	CreatedAt    time.Time        `bson:"created_at"`
	UpdatedAt    time.Time        `bson:"updated_at"`
}

type DriverModel struct {
	ID                  bson.ObjectID    `bson:"_id,omitempty"`
	Name                string           `bson:"name"`
	Email               string           `bson:"email"`
	Password            string           `bson:"password"`
	ProfilePicture      string           `bson:"profile_picture"`
	CarPackage          types.CarPackage `bson:"car_package"`
	CarPlate            string           `bson:"car_plate"`
	CurrentRating       float64          `bson:"current_rating"`
	TotalCompletedTrips int64            `bson:"total_completed_trips"`
	LifetimeRatingAvg   float64          `bson:"lifetime_rating_avg"`
	CreatedAt           time.Time        `bson:"created_at"`
	UpdatedAt           time.Time        `bson:"updated_at"`
}

type RiderModel struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	Name           string        `bson:"name"`
	Email          string        `bson:"email"`
	Password       string        `bson:"password"`
	ProfilePicture string        `bson:"profile_picture"`
	CreatedAt      time.Time     `bson:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at"`
}

type TransactionModel struct {
	ID        bson.ObjectID         `bson:"_id,omitempty"`
	TripID    bson.ObjectID         `bson:"trip_id"`
	Provider  types.PaymentProvider `bson:"provider,omitempty"`
	Amount    int64                 `bson:"amount"`
	Status    types.PaymentStatus   `bson:"status"`
	CreatedAt time.Time             `bson:"created_at"`
	UpdatedAt time.Time             `bson:"updated_at"`
}
