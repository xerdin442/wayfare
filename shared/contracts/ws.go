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
	Trip   types.Trip    `json:"user_id"`
	Driver *types.Driver `json:"driver,omitempty"`
}

type DriverLocationUpdateRequest struct {
	Coords types.Coordinate `json:"coords"`
}

type RiderTripUpdateRequest struct {
	TripID string `json:"tripID"`
}
