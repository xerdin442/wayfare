package models

import (
	"github.com/shopspring/decimal"
	"github.com/xerdin442/wayfare/shared/types"
)

type TripEventModel struct {
	TripID string `ch:"trip_id"`

	// Trip lifecycle details
	Region     string           `ch:"region"`
	DriverID   string           `ch:"driver_id"`
	CarPackage types.CarPackage `ch:"car_package"`
	TripStatus types.TripStatus `ch:"trip_status"`
	Distance   float64          `ch:"distance"`
	PickupLat  float64          `ch:"pickup_lat"`
	PickupLng  float64          `ch:"pickup_lng"`
	Rating     int64            `ch:"rating"`

	// Payment details
	TransactionRef  string                `ch:"transaction_ref"`
	PaymentProvider types.PaymentProvider `ch:"payment_provider"`
	PaymentStatus   types.PaymentStatus   `ch:"payment_status"`
	Amount          decimal.Decimal       `ch:"amount"`
	PlatformFee     decimal.Decimal       `ch:"platform_fee"`
	DriverShare     decimal.Decimal       `ch:"driver_share"`
	DriverTip       decimal.Decimal       `ch:"driver_tip"`
}
