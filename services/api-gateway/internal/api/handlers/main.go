package handlers

import "github.com/xerdin442/wayfare/shared/contracts"

type RouteHandler struct {
	contracts.Base
}

func New(b contracts.Base) *RouteHandler {
	return &RouteHandler{Base: b}
}
