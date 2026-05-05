package models

import (
	"github.com/shopspring/decimal"
	"github.com/xerdin442/wayfare/shared/types"
)

type TripEventModel struct {
	TripID   string `ch:"trip_id"`
	DriverID string `ch:"driver_id"`

	// Trip lifecycle details
	Region                string           `ch:"region"`
	CarPackage            types.CarPackage `ch:"car_package"`
	TripStatus            types.TripStatus `ch:"trip_status"`
	PredictedDurationMins decimal.Decimal  `ch:"predicted_duration_mins"`
	ActualDurationMins    decimal.Decimal  `ch:"actual_duration_mins"`
	DistanceKm            decimal.Decimal  `ch:"distance_km"`
	PickupLat             float64          `ch:"pickup_lat"`
	PickupLng             float64          `ch:"pickup_lng"`
	Rating                int64            `ch:"rating"`
	DriverTip             decimal.Decimal  `ch:"driver_tip"`

	// Payment details
	TransactionRef  string                `ch:"transaction_ref"`
	TransactionType types.TransactionType `ch:"transaction_type"`
	PaymentProvider types.PaymentProvider `ch:"payment_provider"`
	PaymentStatus   types.PaymentStatus   `ch:"payment_status"`
	Amount          decimal.Decimal       `ch:"amount"`
	PlatformFee     decimal.Decimal       `ch:"platform_fee"`
	DriverSplit     decimal.Decimal       `ch:"driver_split"`
}
