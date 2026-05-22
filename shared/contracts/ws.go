package contracts

import (
	"time"

	"github.com/xerdin442/wayfare/shared/types"
)

type WebsocketMessage struct {
	Type string `json:"type"`
	Data any    `json:"data,omitempty"`
}

type TripChatRequest struct {
	TripId    string `json:"tripId"`
	Message   string `json:"message"`
	Recipient string `json:"recipient"`
}

type DriverTripActionRequest struct {
	Trip   types.Trip   `json:"trip"`
	Driver types.Driver `json:"driver,omitempty"`
}

type DriverLocationUpdateRequest struct {
	Coords  types.Coordinate `json:"coords"`
	RiderId string           `json:"riderId,omitempty"`
}

type RegionBoundsRequest struct {
	Pickup types.Coordinate `json:"pickup"`
}

type TripUpdateRequest struct {
	Trip types.Trip `json:"trip"`
}

type TripRatingRequest struct {
	TripId       string `json:"tripId"`
	Rating       int64  `json:"rating"`
	RiderComment string `json:"comment,omitempty"`
}

type DriverAssignedResponse struct {
	Driver types.Driver `json:"driver"`
	Trip   types.Trip   `json:"trip"`
}

type TripRatingRequiredResponse struct {
	TripId      string           `json:"tripId"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
	Date        time.Time        `json:"date"`
}
