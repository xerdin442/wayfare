package handlers

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
)

type RouteHandler struct {
	ws    websocket.Upgrader
	conns sync.Map
	cfg   base.Config
}

func New(c base.Config) *RouteHandler {
	return &RouteHandler{
		cfg: c,
		ws: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				if c.Env.Environment == "development" {
					return true
				}

				origin := r.Header.Get("Origin")
				return origin == c.Env.FrontendUrl
			},
		},
	}
}
