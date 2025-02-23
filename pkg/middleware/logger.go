package middleware

import (
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

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
	allowList map[string]struct{}
	blockList map[string]struct{}
}

func newLogHeaderFilter(allowList []string, blockList []string) logHeaderFilter {
	var filter logHeaderFilter
	if len(allowList) > 0 {
		filter.allowList = make(map[string]struct{})
		for _, header := range allowList {
			filter.allowList[textproto.CanonicalMIMEHeaderKey(header)] = struct{}{}
		}
	}

	if len(blockList) > 0 {
		filter.blockList = make(map[string]struct{})
		for _, header := range blockList {
			filter.blockList[textproto.CanonicalMIMEHeaderKey(header)] = struct{}{}
		}
	}

	return filter
}

// Filter filters the given headers based on the allow list and block list.
//
// Note this WILL modify the given headers.
func (l *logHeaderFilter) Filter(h http.Header) http.Header {
	if len(l.allowList) > 0 {
		for name := range h {
			if _, ok := l.allowList[name]; !ok {
				h.Del(name)
			}
		}
		return h
	}

	if len(l.blockList) > 0 {
		for name := range h {
			if _, ok := l.blockList[name]; ok {
				h.Del(name)
			}
		}
		return h
	}

	return h
}

// NewLogger creates logging middleware for the access log.
func NewLogger(config log.AccessLogConfig, logger log.Logger) gin.HandlerFunc {
	logger = logger.WithSubsystem(logger.Subsystem() + ".access")

	requestHeaderFilter := newLogHeaderFilter(
		config.RequestHeaders.AllowList,
		config.RequestHeaders.BlockList,
	)
	responseHeaderFilter := newLogHeaderFilter(
		config.ResponseHeaders.AllowList,
		config.ResponseHeaders.BlockList,
	)

	level, err := log.ZapLevelFromString(config.Level)
	if err != nil {
		// Validated on boot so must not happen.
		panic("invalid log level")
	}

	return func(c *gin.Context) {
		s := time.Now()

		c.Next()

		if config.Disable {
			// Access log disabled.
			return
		}

		// Ignore internal endpoints.
		if strings.HasPrefix(c.Request.URL.Path, "/_piko") {
			return
		}

		// Note filter will modify the request/response headers, though
		// they have already been written so it doesn't matter.
		requestHeaders := requestHeaderFilter.Filter(c.Request.Header)
		responseHeaders := responseHeaderFilter.Filter(c.Writer.Header())

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

		recordLevel := level
		// If the response is a server error, increase the log level to a
		// minimum of 'warn'.
		if c.Writer.Status() >= http.StatusInternalServerError && recordLevel < zapcore.WarnLevel {
			recordLevel = zapcore.WarnLevel
		}

		logger.Log(recordLevel, "request", zap.Any("request", req))
	}
}
