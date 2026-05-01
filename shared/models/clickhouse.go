package models

import (
	"time"

	"github.com/shopspring/decimal"
	"github.com/xerdin442/wayfare/shared/types"
)

type TripLifecycleEventModel struct {
	TripID     string           `ch:"trip_id"`
	RegionID   string           `ch:"region_id"`
	CarPackage types.CarPackage `ch:"car_package"`
	TripStatus types.TripStatus `ch:"trip_status"`
	Distance   float64          `ch:"distance"`
	PickupLat  float64          `ch:"pickup_lat"`
	PickupLng  float64          `ch:"pickup_lng"`
	Rating     int64            `ch:"rating"`
	Timestamp  time.Time        `ch:"timestamp"`
}

type PaymentEventModel struct {
	TransactionRef string                `ch:"transaction_ref"`
	TripID         string                `ch:"trip_id"`
	RegionID       string                `ch:"region_id"`
	DriverID       string                `ch:"driver_id"`
	Provider       types.PaymentProvider `ch:"payment_provider"`
	PaymentStatus  types.PaymentStatus   `ch:"payment_status"`
	Amount         decimal.Decimal       `ch:"amount"`
	PlatformFee    decimal.Decimal       `ch:"platform_fee"`
	DriverShare    decimal.Decimal       `ch:"driver_share"`
	DriverTip      decimal.Decimal       `ch:"driver_tip"`
	Timestamp      time.Time             `ch:"timestamp"`
}
