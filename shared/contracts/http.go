package contracts

import (
	"github.com/xerdin442/wayfare/shared/types"
)

type APIResponse struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type PreviewTripRequest struct {
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}

type PreviewTripResponse struct {
	Route     types.Route      `json:"route"`
	RideFares []types.RideFare `json:"rideFares"`
}

type StartTripRequest struct {
	RideFareID string `json:"rideFareID"`
}

type StartTripResponse struct {
	TripID string `json:"tripID"`
}
