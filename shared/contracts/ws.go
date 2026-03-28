package contracts

import (
	"encoding/json"

	"github.com/xerdin442/wayfare/shared/messaging"
	"github.com/xerdin442/wayfare/shared/types"
)

type WSOutgoingMessage struct {
	Type messaging.AmqpEvent `json:"type"`
	Data any                 `json:"data,omitempty"`
}

type WSIncomingMessage struct {
	Type messaging.AmqpEvent `json:"type"`
	Data *json.RawMessage    `json:"data,omitempty"`
}

type DriverTripActionRequest struct {
	Trip   types.Trip    `json:"user_id"`
	Driver *types.Driver `json:"driver,omitempty"`
}

type DriverLocationUpdateRequest struct {
	Location types.Coordinate `json:"location"`
	Geohash  string           `json:"geohash"`
}

type RiderTripUpdateRequest struct {
	TripID string `json:"tripID"`
}
