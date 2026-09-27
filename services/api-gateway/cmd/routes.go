package main

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/handlers"
	"github.com/xerdin442/wayfare/services/api-gateway/internal/api/middleware"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

const (
	livenessPath        = "/livez"
	paymentCallbackPath = "/api/v1/payment/callback"
)

func (app *application) routes() http.Handler {
	r := gin.New()
	m := middleware.New(app.config)
	h := handlers.New(app.config)

	if err := r.SetTrustedProxies(app.config.Env.TrustedProxies); err != nil {
		log.Fatal().Err(err).Msg("Invalid TRUSTED_PROXIES value")
	}

	r.Use(m.CustomRequestLogger())
	r.Use(gin.Recovery())

	corsConfig := cors.DefaultConfig()
	frontendUrl := app.config.Env.FrontendUrl
	if app.config.Env.Environment == "production" {
		corsConfig.AllowOrigins = []string{
			fmt.Sprintf("https://%s", frontendUrl),
			fmt.Sprintf("https://www.%s", frontendUrl),
		}
	} else {
		corsConfig.AllowOrigins = []string{"http://localhost:3000"}
	}
	corsConfig.AllowCredentials = true
	corsConfig.AddAllowHeaders("Authorization", "X-User-Role")
	r.Use(cors.New(corsConfig))

	// Webhooks are authenticated by signature, and health probes come from the platform
	r.Use(m.RateLimiters(paymentCallbackPath, livenessPath)...)

	// Liveness check
	r.GET(livenessPath, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Websockets
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
		auth.POST("/refresh", otelgin.Middleware("auth.refresh"), h.HandleRefresh)
		auth.POST("/logout", m.JwtGuard(), otelgin.Middleware("auth.logout"), h.HandleLogout)
	}

	user := v1.Group("/user", m.JwtGuard())
	{
		user.GET("/profile", otelgin.Middleware("user.profile"), h.HandleUserProfile)
	}

	driver := v1.Group("/driver", m.JwtGuard())
	{
		driver.POST("/returns/pay", otelgin.Middleware("driver.pay_returns"), h.HandleInitiateCheckout)
	}

	trip := v1.Group("/trip", m.JwtGuard())
	{
		trip.POST("/start", otelgin.Middleware("trip.start"), h.HandleStartTrip)
		trip.POST("/preview", otelgin.Middleware("trip.preview"), h.HandleTripPreview)
		trip.POST("/:id/pay", otelgin.Middleware("trip.checkout"), h.HandleInitiateCheckout)
		trip.GET("/:id/chat", otelgin.Middleware("trip.chat"), h.HandleTripChat)
		trip.GET("/history", otelgin.Middleware("trip.history"), h.HandleTripHistory)
	}

	r.POST(paymentCallbackPath, otelgin.Middleware("payment.callback"), h.HandlePaymentCallback)

	return r
}
