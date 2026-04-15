package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/handlers"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
)

func (app *application) routes() http.Handler {
	r := gin.New()
	m := middleware.New(app.config)
	h := handlers.New(app.config)

	r.Use(m.CustomRequestLogger())
	r.Use(m.RateLimiters()...)
	r.Use(gin.Recovery())

	// Liveness check
	r.GET("/livez", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Websocket handlers
	r.Group("/ws")
	{
		r.GET("/drivers", h.HandleDriversConnection)
		r.GET("/riders", h.HandleRidersConnection)
	}

	v1 := r.Group("/api/v1")

	v1.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello from the API Gateway!")
	})

	auth := v1.Group("/auth")
	{
		auth.POST("/signup", h.HandleSignup)
		auth.POST("/login", h.HandleLogin)
		auth.POST("/logout", m.JwtGuard(), h.HandleLogout)
	}

	trip := v1.Group("/trip", m.JwtGuard())
	{
		trip.POST("/start", h.HandleStartTrip)
		trip.POST("/preview", h.HandleTripPreview)
	}

	return r
}
