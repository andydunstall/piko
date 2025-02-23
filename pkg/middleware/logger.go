package middleware

import (
	"net/http"
	"net/textproto"
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

type logHeaderFilter struct {
	allowList map[string]string
	blockList map[string]string
}

type loggerConfig struct {
	RequestHeader  logHeaderFilter
	ResponseHeader logHeaderFilter
}

// NewLogger creates logging middleware that logs every request.
func NewLogger(config log.AccessLogConfig, l log.Logger) gin.HandlerFunc {
	l = l.WithSubsystem(l.Subsystem() + ".access")

	lc := newLoggerConfig(config)
	return func(c *gin.Context) {
		s := time.Now()

		c.Next()

		// Ignore internal endpoints.
		if strings.HasPrefix(c.Request.URL.Path, "/_piko") {
			return
		}

		requestHeaders := lc.RequestHeader.Filter(c.Request.Header)
		responseHeaders := lc.ResponseHeader.Filter(c.Writer.Header())

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
			l.Warn("request", zap.Any("request", req))
		} else if config.Disable {
			l.Debug("request", zap.Any("request", req))
		} else {
			l.Info("request", zap.Any("request", req))
		}
	}
}

func (l *logHeaderFilter) New(allowList []string, blockList []string) {
	if len(allowList) > 0 {
		l.allowList = make(map[string]string)
		for _, el := range allowList {
			h := textproto.CanonicalMIMEHeaderKey(el)
			l.allowList[h] = h
		}
	}

	if len(blockList) > 0 {
		l.blockList = make(map[string]string)
		for _, el := range blockList {
			h := textproto.CanonicalMIMEHeaderKey(el)
			l.blockList[h] = h
		}
	}
}

func (l *logHeaderFilter) Filter(h http.Header) http.Header {
	if len(l.allowList) > 0 {
		for name := range h {
			// Use the map created during validation to hasten lookups.
			if _, ok := l.allowList[name]; !ok {
				h.Del(name)
			}
		}
		return h
	}

	if len(l.blockList) > 0 {
		for _, blocked := range l.blockList {
			h.Del(blocked)
		}
		return h
	}

	return h
}

func newLoggerConfig(c log.AccessLogConfig) loggerConfig {
	l := loggerConfig{}
	l.RequestHeader.New(c.RequestHeaders.Allowlist, c.RequestHeaders.Blocklist)
	l.ResponseHeader.New(c.ResponseHeaders.Allowlist, c.ResponseHeaders.Blocklist)
	return l
}
