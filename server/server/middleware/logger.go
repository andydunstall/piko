package middleware

import (
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewLogger creates logging middleware that logs every request.
func NewLogger(logger log.Logger) gin.HandlerFunc {
	logger = logger.WithSubsystem(logger.Subsystem() + ".route")
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		if c.Request.URL.RawQuery != "" {
			path = path + "?" + c.Request.URL.RawQuery
		}

		// Process request
		c.Next()

		logger.Debug(
			"http request",
			zap.String("method", c.Request.Method),
			zap.Int("status", c.Writer.Status()),
			zap.String("path", path),
			zap.Int64("latency", time.Since(start).Milliseconds()),
			zap.String("client-ip", c.ClientIP()),
			zap.Int("resp-size", c.Writer.Size()),
		)
	}
}
