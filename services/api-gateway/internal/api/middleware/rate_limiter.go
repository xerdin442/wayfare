package middleware

import (
	"net/http"
	"slices"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	"github.com/ulule/limiter/v3"
	mgin "github.com/ulule/limiter/v3/drivers/middleware/gin"
	"github.com/ulule/limiter/v3/drivers/store/redis"
)

func (m *Middleware) RateLimiters(exemptPaths ...string) []gin.HandlerFunc {
	limitHandler := func(c *gin.Context) {
		log.Warn().Msgf("Rate-limited requests from IP: %s", c.ClientIP())

		c.AbortWithStatusJSON(
			http.StatusTooManyRequests,
			gin.H{"error": "Too many requests. Please try again later."},
		)
	}

	skipOptions := func(inner gin.HandlerFunc) gin.HandlerFunc {
		return func(c *gin.Context) {
			if c.Request.Method == http.MethodOptions || slices.Contains(exemptPaths, c.FullPath()) {
				c.Next()
				return
			}
			inner(c)
		}
	}

	newLimiter := func(prefix string, rate limiter.Rate) gin.HandlerFunc {
		store, err := redis.NewStoreWithOptions(m.cfg.Cache, limiter.StoreOptions{Prefix: prefix})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create Redis store for rate limiter")
		}

		return skipOptions(mgin.NewMiddleware(
			limiter.New(store, rate),
			mgin.WithLimitReachedHandler(limitHandler),
		))
	}

	secondLimiter := newLimiter("limiter:second", limiter.Rate{
		Period: 1 * time.Second,
		Limit:  10,
	})

	minuteLimiter := newLimiter("limiter:minute", limiter.Rate{
		Period: 1 * time.Minute,
		Limit:  60,
	})

	return []gin.HandlerFunc{secondLimiter, minuteLimiter}
}
