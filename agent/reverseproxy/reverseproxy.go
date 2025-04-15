package reverseproxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
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

	dialer := &net.Dialer{
		Timeout:   conf.Timeout,
		KeepAlive: conf.HTTPClient.KeepAliveTimeout,
	}
	// Same as http.DefaultTransport with custom TLS client config.
	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       conf.HTTPClient.IdleConnTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          conf.HTTPClient.MaxIdleConns,
		DisableCompression:    conf.HTTPClient.DisableCompression,
	}
	tlsClientConfig, err := conf.TLS.Load()
	if err != nil {
		// Validated on boot so should never happen.
		panic("invalid tls config: " + err.Error())
	}
	transport.TLSClientConfig = tlsClientConfig

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = transport
	proxy.ErrorLog = logger.StdLogger(zapcore.WarnLevel)
	rp := &ReverseProxy{
		proxy:   proxy,
		timeout: conf.Timeout,
		logger:  logger,
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
