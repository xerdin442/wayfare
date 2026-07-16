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
	Addresses   []string `bson:"addresses"` // [0] = pickup, [1] = destination
	Duration    float64  `bson:"duration"`
	Distance    float64  `bson:"distance"`
	Polyline    string   `bson:"polyline,omitempty"`
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
	BaseFee       int64            `bson:"base_fee"`
	PerKm         int64            `bson:"per_km"`
	PerMinute     int64            `bson:"per_minute"`
	AfterHoursFee int64            `bson:"after_hours_fee"`
	MinFare       int64            `bson:"min_fare"`
	CreatedAt     time.Time        `bson:"created_at"`
	UpdatedAt     time.Time        `bson:"updated_at"`
}

type RideFareModel struct {
	ID         bson.ObjectID    `bson:"_id,omitempty"`
	UserID     bson.ObjectID    `bson:"user_id"`
	RegionID   bson.ObjectID    `bson:"region_id"`
	CarPackage types.CarPackage `bson:"car_package"`
	Amount     int64            `bson:"amount"`
	ExpiresAt  time.Time        `bson:"expires_at"`
	Route      RouteDetails     `bson:"route"`
	CreatedAt  time.Time        `bson:"created_at"`
	UpdatedAt  time.Time        `bson:"updated_at"`
}

type TripModel struct {
	ID           bson.ObjectID    `bson:"_id,omitempty"`
	DriverID     bson.ObjectID    `bson:"driver_id,omitempty"`
	UserID       bson.ObjectID    `bson:"user_id"`
	Region       string           `bson:"region"`
	Status       types.TripStatus `bson:"status"`
	RideFare     int64            `bson:"ride_fare"`
	CarPackage   types.CarPackage `bson:"car_package"`
	Route        RouteDetails     `bson:"route"`
	StartedAt    time.Time        `bson:"started_at,omitempty"`
	EndedAt      time.Time        `bson:"ended_at,omitempty"`
	Rating       int32            `bson:"rating,omitempty"`
	RiderComment string           `bson:"rider_comment,omitempty"`
	DriverTip    int64            `bson:"driver_tip,omitempty"`
	CreatedAt    time.Time        `bson:"created_at"`
	UpdatedAt    time.Time        `bson:"updated_at"`
}

type DriverModel struct {
	ID                    bson.ObjectID      `bson:"_id,omitempty"`
	Name                  string             `bson:"name"`
	Email                 string             `bson:"email"`
	Phone                 string             `bson:"phone"`
	Password              string             `bson:"password"`
	ProfilePicture        string             `bson:"profile_picture"`
	VerificationPhotos    []string           `bson:"verification_photos"`
	IsVerified            bool               `bson:"is_verified"`
	CarPackage            types.CarPackage   `bson:"car_package"`
	CarPlate              string             `bson:"car_plate"`
	CarModel              string             `bson:"car_model"`
	CarColor              string             `bson:"car_color"`
	CurrentRating         float64            `bson:"current_rating"`
	TotalCompletedTrips   int32              `bson:"total_completed_trips"`
	LifetimeRatingAvg     float64            `bson:"lifetime_rating_avg"`
	AvailableBalance      int64              `bson:"available_balance"`
	PendingPayout         int64              `bson:"pending_payout"`
	PendingReturns        int64              `bson:"pending_returns"`
	OutstandingReturns    int64              `bson:"outstanding_returns"`
	TransferRecipientCode string             `bson:"transfer_recipient_code"`
	Tier                  types.DriverTier   `bson:"tier"`
	Status                types.DriverStatus `bson:"status"`
	CreatedAt             time.Time          `bson:"created_at"`
	UpdatedAt             time.Time          `bson:"updated_at"`
}

type RiderModel struct {
	ID             bson.ObjectID `bson:"_id,omitempty"`
	Name           string        `bson:"name"`
	Email          string        `bson:"email"`
	Phone          string        `bson:"phone"`
	Password       string        `bson:"password"`
	ProfilePicture string        `bson:"profile_picture"`
	CreatedAt      time.Time     `bson:"created_at"`
	UpdatedAt      time.Time     `bson:"updated_at"`
}

type TransactionModel struct {
	ID                  bson.ObjectID         `bson:"_id,omitempty"`
	TripID              bson.ObjectID         `bson:"trip_id,omitempty"`
	DriverID            bson.ObjectID         `bson:"driver_id,omitempty"`
	DriverRecipientCode string                `bson:"driver_recipient_code,omitempty"`
	Provider            types.PaymentProvider `bson:"provider,omitempty"`
	Amount              int64                 `bson:"amount"`
	Status              types.PaymentStatus   `bson:"status"`
	Type                types.TransactionType `bson:"type"`
	TransferFee         int64                 `bson:"transfer_fee,omitempty"`
	CreatedAt           time.Time             `bson:"created_at"`
	UpdatedAt           time.Time             `bson:"updated_at"`
}
