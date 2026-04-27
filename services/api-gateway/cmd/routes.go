package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/handlers"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
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
	ws := r.Group("/ws")
	{
		ws.GET("/drivers", otelgin.Middleware("ws.drivers"), h.HandleDriversConnection)
		ws.GET("/riders", otelgin.Middleware("ws.riders"), h.HandleRidersConnection)
	}

	v1 := r.Group("/api/v1")

	v1.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Hello from the API Gateway!")
	})

	auth := v1.Group("/auth")
	{
		auth.POST("/signup", otelgin.Middleware("auth.signup"), h.HandleSignup)
		auth.POST("/login", otelgin.Middleware("auth.login"), h.HandleLogin)
		auth.POST("/logout", m.JwtGuard(), otelgin.Middleware("auth.logout"), h.HandleLogout)
	}

	user := v1.Group("/user", m.JwtGuard())
	{
		user.GET("/profile", otelgin.Middleware("user.profile"), h.HandleUserProfile)
	}

	trip := v1.Group("/trip", m.JwtGuard())
	{
		trip.POST("/start", otelgin.Middleware("trip.start"), h.HandleStartTrip)
		trip.POST("/preview", otelgin.Middleware("trip.preview"), h.HandleTripPreview)
		trip.POST("/:id/pay", otelgin.Middleware("trip.pay"), h.HandleInitiatePayment)
	}

	v1.POST("/payment/callback", m.JwtGuard(), otelgin.Middleware("payment.callback"), h.HandlePaymentCallback)

	return r
}
