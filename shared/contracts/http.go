package contracts

import (
	"github.com/redis/go-redis/v9"
	"github.com/xerdin442/wayfare/shared/secrets"
)

type Base struct {
	Env   *secrets.Secrets
	Cache *redis.Client
}

// APIResponse is the response structure for API requests
type APIResponse struct {
	Data  any       `json:"data,omitempty"`
	Error *APIError `json:"error,omitempty"`
}

// APIError is the error structure for API requests
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
