package contracts

import (
	"github.com/redis/go-redis/v9"
	"github.com/xerdin442/wayfare/shared/secrets"
	"github.com/xerdin442/wayfare/shared/types"
)

type Base struct {
	Env   *secrets.Secrets
	Cache *redis.Client
}

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
	Route     types.Route       `json:"route"`
	RideFares []types.RouteFare `json:"rideFares"`
}

type StartTripRequest struct {
	RideFareID string `json:"rideFareID"`
}

type StartTripResponse struct {
	TripID string `json:"tripID"`
}
