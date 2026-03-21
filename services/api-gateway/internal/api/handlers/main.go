package handlers

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/xerdin442/wayfare/shared/contracts"
)

type RouteHandler struct {
	ws    websocket.Upgrader
	conns sync.Map
	contracts.Base
}

func New(b contracts.Base) *RouteHandler {
	return &RouteHandler{
		Base: b,
		ws: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				if b.Env.Environment == "development" {
					return true
				}

				origin := r.Header.Get("Origin")
				return origin == b.Env.FrontendUrl
			},
		},
	}
}
