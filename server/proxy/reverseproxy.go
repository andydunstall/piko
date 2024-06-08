package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/upstream"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey int

const (
	endpointContextKey contextKey = iota
	upstreamContextKey
)

type ReverseProxy struct {
	upstreams upstream.Manager

	proxy *httputil.ReverseProxy

	timeout time.Duration

	logger log.Logger
}

func NewReverseProxy(
	upstreams upstream.Manager,
	timeout time.Duration,
	logger log.Logger,
) *ReverseProxy {
	rp := &ReverseProxy{
		upstreams: upstreams,
		timeout:   timeout,
		logger:    logger,
	}

	rp.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Context().Value(endpointContextKey).(string)

			req.Header.Set("x-piko-forward", "true")
		},
		Transport: &http.Transport{
			DialContext: rp.dialUpstream,
			// 'connections' to the upstream are multiplexed over a single TCP
			// connection so theres no overhead to creating new connections,
			// therefore it doesn't make sense to keep them alive.
			DisableKeepAlives: true,
		},
		ErrorLog:     logger.StdLogger(zapcore.WarnLevel),
		ErrorHandler: rp.errorHandler,
	}

	return rp
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if p.timeout != 0 {
		ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
		defer cancel()

		r = r.WithContext(ctx)
	}

	endpointID := EndpointIDFromRequest(r)
	if endpointID == "" {
		p.logger.Warn("request missing endpoint id")

		_ = errorResponse(w, http.StatusBadRequest, "missing endpoint id")
		return
	}

	ctx := context.WithValue(r.Context(), endpointContextKey, endpointID)
	r = r.WithContext(ctx)

	// Whether the request was forwarded from another Piko node.
	forwarded := r.Header.Get("x-piko-forward") == "true"

	// If there is a connected upstream, attempt to forward the request to one
	// of those upstreams. Note this includes remote nodes that are reporting
	// they have an available upstream. We don't allow multiple hops, so if
	// forwarded is true we only select from local nodes.
	upstream, ok := p.upstreams.Select(endpointID, !forwarded)
	if !ok {
		_ = errorResponse(w, http.StatusBadGateway, "no available upstreams")
		return
	}

	// Add the upstream to the context to pass to 'DialContext'.
	ctx = context.WithValue(r.Context(), upstreamContextKey, upstream)
	r = r.WithContext(ctx)

	p.proxy.ServeHTTP(w, r)
}

func (p *ReverseProxy) dialUpstream(ctx context.Context, _, _ string) (net.Conn, error) {
	// As a bit of a hack to work with http.Transport, we add the upstream
	// to the dial context.
	upstream := ctx.Value(upstreamContextKey).(upstream.Upstream)
	return upstream.Dial()
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

// EndpointIDFromRequest returns the endpoint ID from the HTTP request, or an
// empty string if no endpoint ID is specified.
//
// This will check both the 'x-piko-endpoint' header and 'Host' header, where
// x-piko-endpoint takes precedence.
func EndpointIDFromRequest(r *http.Request) string {
	endpointID := r.Header.Get("x-piko-endpoint")
	if endpointID != "" {
		return endpointID
	}

	host := r.Host
	if host != "" && strings.Contains(host, ".") {
		// If a host is given and contains a separator, use the bottom-level
		// domain as the endpoint ID.
		//
		// Such as if the domain is 'xyz.piko.example.com', then 'xyz' is the
		// endpoint ID.
		return strings.Split(host, ".")[0]
	}

	return ""
}
