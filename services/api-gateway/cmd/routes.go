package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/handlers"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
)

func (app *application) routes() http.Handler {
	r := gin.New()
	m := middleware.New(app.Base)
	h := handlers.New(app.Base)

	r.Use(m.CustomRequestLogger())
	r.Use(m.RateLimiters()...)
	r.Use(gin.Recovery())

	v1 := r.Group("/api/v1")

	v1.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "Hello from the API Gateway!")
	})

	trip := v1.Group("/trip", m.JwtGuard())
	{
		trip.POST("/start")
		trip.POST("/preview", h.HandleTripPreview)
	}

	return r
}
