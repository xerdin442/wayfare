package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/redis"
)

func (m *Middleware) RateLimiters() []gin.HandlerFunc {
	store, err := redis.NewStore(m.cfg.Cache)
	if err != nil {
		log.Fatal().Msg("Failed to create Redis store for rate limiter")
	}

	limitHandler := func(c *gin.Context) {
		log.Warn().Msgf("Rate-limited requests from IP: %s", c.ClientIP())

		c.AbortWithStatusJSON(
			http.StatusTooManyRequests,
			gin.H{"error": "Too many requests. Please try again later."},
		)
	}

	skipOptions := func(inner gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if c.Request.Method == http.MethodOptions {
				c.Next()
				return
			}
			inner(c)
		}
	}

	secondLimiter := skipOptions(mgin.NewMiddleware(
		limiter.New(store, limiter.Rate{
			Period: 1 * time.Second,
			Limit:  10,
		}),
		mgin.WithLimitReachedHandler(limitHandler),
	))

	minuteLimiter := skipOptions(mgin.NewMiddleware(
		limiter.New(store, limiter.Rate{
			Period: 1 * time.Minute,
			Limit:  60,
		}),
		mgin.WithLimitReachedHandler(limitHandler),
	))

	return []gin.HandlerFunc{secondLimiter, minuteLimiter}
}
