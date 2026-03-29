package middleware

import (
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
)

type Middleware struct {
	cfg *base.Config
}

func New(c *base.Config) *Middleware {
	return &Middleware{cfg: c}
}
