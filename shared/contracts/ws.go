package contracts

import (
	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/types"
)

type WebsocketMessage struct {
	Type messaging.AmqpEvent `json:"type"`
	Data any                 `json:"data,omitempty"`
}

type DriverTripActionRequest struct {
	Trip   types.Trip   `json:"trip"`
	Driver types.Driver `json:"driver"`
}

type DriverLocationUpdateRequest struct {
	Coords types.Coordinate `json:"coords"`
}

type TripUpdateRequest struct {
	Trip types.Trip `json:"trip"`
}

type CashPaymentRequest struct {
	TripID       string `json:"tripId"`
	TripRating   int64  `json:"tripRating"`
	RiderComment string `json:"riderComment,omitempty"`
}

type DriverAssignedResponse struct {
	Driver types.Driver `json:"driver"`
	Trip   types.Trip   `json:"trip"`
}
