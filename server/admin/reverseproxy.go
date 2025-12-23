package admin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dragonflydb/piko/pkg/log"
)

type contextKey int

const (
	hostContextKey contextKey = iota
)

type ReverseProxy struct {
	proxy *httputil.ReverseProxy

	logger log.Logger
}

func NewReverseProxy(logger log.Logger) *ReverseProxy {
	rp := &ReverseProxy{
		logger: logger,
	}

	rp.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Context().Value(hostContextKey).(string)
		},
		ErrorLog:     logger.StdLogger(zapcore.WarnLevel),
		ErrorHandler: rp.errorHandler,
	}

	return rp
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), time.Second*15)
	defer cancel()

	r = r.WithContext(ctx)

	p.proxy.ServeHTTP(w, r)
}

func (p *ReverseProxy) errorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	p.logger.Warn("proxy request", zap.Error(err))

	if errors.Is(err, context.DeadlineExceeded) {
		_ = errorResponse(w, http.StatusGatewayTimeout, "upstream timeout")
		return
	}
	_ = errorResponse(w, http.StatusBadGateway, "upstream unreachable")
}

type errorMessage struct {
	Error string `json:"error"`
}

func errorResponse(w http.ResponseWriter, statusCode int, message string) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(statusCode)

	m := &errorMessage{
		Error: message,
	}
	return json.NewEncoder(w).Encode(m)
}
