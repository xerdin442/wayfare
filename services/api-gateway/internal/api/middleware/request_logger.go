package middleware

import (
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func (m *Middleware) CustomRequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		// Display log level based on HTTP status
		status := c.Writer.Status()
		event := log.Info()
		if status >= 400 && status < 500 {
			event = log.Warn()
		} else if status >= 500 {
			event = log.Error()
		}

		latency := fmt.Sprintf("%.2fms", float64(time.Since(start).Milliseconds()))

		event.
			Str("method", c.Request.Method).
			Int("status", status).
			Str("path", path).
			Str("query", query).
			Str("ip", c.ClientIP()).
			Str("latency", latency).
			Msg("Request processed")
	}
}
