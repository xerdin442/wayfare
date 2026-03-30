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

type DriverAssignedResponse struct {
	Driver types.Driver `json:"driver"`
	TripID string       `json:"tripID"`
}
