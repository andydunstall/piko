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

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/upstream"
)

type contextKey int

const (
	endpointContextKey contextKey = iota
	upstreamContextKey
)

// HTTPProxy proxies HTTP traffic to upsteam listeners.
type HTTPProxy struct {
	upstreams upstream.Manager

	proxy *httputil.ReverseProxy

	timeout time.Duration

	logger log.Logger
}

func NewHTTPProxy(
	upstreams upstream.Manager,
	timeout time.Duration,
	logger log.Logger,
) *HTTPProxy {
	rp := &HTTPProxy{
		upstreams: upstreams,
		timeout:   timeout,
		logger:    logger.WithSubsystem("proxy.http"),
	}

	rp.proxy = &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = req.Context().Value(endpointContextKey).(string)
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

func (p *HTTPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	endpointID := EndpointIDFromRequest(r)
	if endpointID == "" {
		p.logger.Warn("request missing endpoint id")

		_ = errorResponse(w, http.StatusBadRequest, "missing endpoint id")
		return
	}

	// Whether the request was forwarded from another Piko node.
	forwarded := r.Header.Get("x-piko-forward") == "true"

	// If there is a connected upstream, attempt to forward the request to one
	// of those upstreams. Note this includes remote nodes that are reporting
	// they have an available upstream. We don't allow multiple hops, so if
	// forwarded is true we only select from local nodes.
	upstream, ok := p.upstreams.Select(endpointID, !forwarded)
	if !ok {
		p.logger.Warn(
			"no available upstreams",
			zap.String("endpoint-id", endpointID),
		)

		_ = errorResponse(w, http.StatusBadGateway, "no available upstreams")
		return
	}

	p.ServeHTTPWithUpstream(w, r, endpointID, upstream)
}

func (p *HTTPProxy) ServeHTTPWithUpstream(
	w http.ResponseWriter,
	r *http.Request,
	endpointID string,
	upstream upstream.Upstream,
) {
	if p.timeout != 0 {
		ctx, cancel := context.WithTimeout(r.Context(), p.timeout)
		defer cancel()

		r = r.WithContext(ctx)
	}

	r.Header.Set("x-piko-forward", "true")

	r = r.WithContext(context.WithValue(r.Context(), endpointContextKey, endpointID))

	// Add the upstream to the context to pass to 'DialContext'.
	r = r.WithContext(context.WithValue(r.Context(), upstreamContextKey, upstream))

	p.proxy.ServeHTTP(w, r)
}

func (p *HTTPProxy) dialUpstream(ctx context.Context, _, _ string) (net.Conn, error) {
	// As a bit of a hack to work with http.Transport, we add the upstream
	// to the dial context.
	upstream := ctx.Value(upstreamContextKey).(upstream.Upstream)
	return upstream.Dial()
}

func (p *HTTPProxy) errorHandler(w http.ResponseWriter, _ *http.Request, err error) {
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

	// Strip the port if given.
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		host = r.Host
	}

	if host == "" {
		return ""
	}
	if net.ParseIP(host) != nil {
		// Ignore IP addresses.
		return ""
	}
	if strings.Contains(host, ".") {
		// If a host is given and contains a separator, use the bottom-level
		// domain as the endpoint ID.
		//
		// Such as if the domain is 'xyz.piko.example.com', then 'xyz' is the
		// endpoint ID.
		return strings.Split(host, ".")[0]
	}

	return ""
}
