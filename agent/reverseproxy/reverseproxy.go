package reverseproxy

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httputil"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
)

type ReverseProxy struct {
	proxy *httputil.ReverseProxy

	timeout time.Duration

	logger log.Logger
}

func NewReverseProxy(conf config.ListenerConfig, logger log.Logger) *ReverseProxy {
	u, ok := conf.URL()
	if !ok {
		// We've already verified the address on boot so don't need to handle
		// the error.
		panic("invalid addr: " + conf.Addr)
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.ErrorLog = logger.StdLogger(zapcore.WarnLevel)
	rp := &ReverseProxy{
		proxy:   proxy,
		timeout: conf.Timeout,
		logger:  logger,
	}
	if conf.InsecureSkipVerify {
		proxy.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}
	proxy.ErrorHandler = rp.errorHandler
	return rp
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.timeout != 0 && r.Header.Get("upgrade") != "websocket" {
		ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
		defer cancel()

		r = r.WithContext(ctx)
	}

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
