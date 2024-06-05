package endpoint

import (
	"net/http"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
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

// NewLoggerMiddleware creates logging middleware that logs every request.
func NewLoggerMiddleware(accessLog bool, logger log.Logger) gin.HandlerFunc {
	logger = logger.WithSubsystem("endpoint.access")
	return func(c *gin.Context) {
		s := time.Now()

		c.Next()

		req := &loggedRequest{
			Proto:           c.Request.Proto,
			Method:          c.Request.Method,
			Host:            c.Request.Host,
			Path:            c.Request.URL.Path,
			RequestHeaders:  c.Request.Header,
			ResponseHeaders: c.Writer.Header(),
			Status:          c.Writer.Status(),
			Duration:        time.Since(s).String(),
		}
		if c.Writer.Status() > http.StatusInternalServerError {
			logger.Warn("request", zap.Any("request", req))
		} else if accessLog {
			logger.Info("request", zap.Any("request", req))
		} else {
			logger.Debug("request", zap.Any("request", req))
		}
	}
}
