package handlers

import (
	"fmt"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/base"
)

type RouteHandler struct {
	ws  websocket.Upgrader
	cfg *base.Config
}

func New(c *base.Config) *RouteHandler {
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
				frontendUrl := c.Env.FrontendUrl
				return origin == fmt.Sprintf("https://%s", frontendUrl) || origin == fmt.Sprintf("https://www.%s", frontendUrl)
			},
		},
	}
}
