package middleware

import "github.com/xerdin442/wayfare/shared/contracts"

type Middleware struct {
	contracts.Base
}

func New(b contracts.Base) *Middleware {
	return &Middleware{Base: b}
}
