package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/log"
)

type loggedRequest struct {
	Proto           string      `json:"proto"`
	Method          string      `json:"method"`
	Host            string      `json:"host"`
	Path            string      `json:"path"`
	RequestHeaders  http.Header `json:"request_headers"`
	ResponseHeaders http.Header `json:"response_headers"`
	Status          int         `json:"status"`
	Duration        string      `json:"duration"`
}

// NewLogger creates logging middleware that logs every request.
func NewLogger(accessLogConfig log.AccessLogConfig, logger log.Logger) gin.HandlerFunc {
	logger = logger.WithSubsystem(logger.Subsystem() + ".access")
	return func(c *gin.Context) {
		s := time.Now()

		c.Next()

		// Ignore internal endpoints.
		if strings.HasPrefix(c.Request.URL.Path, "/_piko") {
			return
		}

		requestHeaders := accessLogConfig.RequestHeaders.Filter(c.Request.Header)
		responseHeaders := accessLogConfig.ResponseHeaders.Filter(c.Writer.Header())

		req := &loggedRequest{
			Proto:           c.Request.Proto,
			Method:          c.Request.Method,
			Host:            c.Request.Host,
			Path:            c.Request.URL.Path,
			RequestHeaders:  requestHeaders,
			ResponseHeaders: responseHeaders,
			Status:          c.Writer.Status(),
			Duration:        time.Since(s).String(),
		}
		if c.Writer.Status() >= http.StatusInternalServerError {
			logger.Warn("request", zap.Any("request", req))
		} else if accessLogConfig.Enabled {
			logger.Info("request", zap.Any("request", req))
		} else {
			logger.Debug("request", zap.Any("request", req))
		}
	}
}
