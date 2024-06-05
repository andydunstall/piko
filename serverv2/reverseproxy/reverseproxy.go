package reverseproxy

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/serverv2/upstream"
	"go.uber.org/zap"
)

type contextKey int

const (
	upstreamContextKey contextKey = iota
)

type UpstreamManager interface {
	Select(endpointID string, allowForward bool) (upstream.Upstream, bool)
}

type ReverseProxy struct {
	upstreams UpstreamManager

	// upstreamTransport is the transport to forward requests to upstream
	// connections.
	upstreamTransport *http.Transport

	logger log.Logger
}

func NewReverseProxy(upstreams UpstreamManager, logger log.Logger) *ReverseProxy {
	proxy := &ReverseProxy{
		upstreams: upstreams,
		logger:    logger,
	}

	proxy.upstreamTransport = &http.Transport{
		DialContext: proxy.dialUpstream,
		// 'connections' to the upstream are multiplexed over a single TCP
		// connection so theres no overhead to creating new connections,
		// therefore it doesn't make sense to keep them alive.
		DisableKeepAlives: true,
	}

	return proxy
}

func (p *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Whether the request was forwarded from another Piko node.
	forwarded := r.Header.Get("x-piko-forward") == "true"

	logger := p.logger.With(
		zap.String("method", r.Method),
		zap.String("host", r.Host),
		zap.String("path", r.URL.Path),
		zap.Bool("forwarded", forwarded),
	)

	// TODO(andydunstall): Add a timeout to ctx.

	endpointID := EndpointIDFromRequest(r)
	if endpointID == "" {
		logger.Warn("request missing endpoint id")

		if err := errorResponse(
			w, http.StatusBadRequest, "missing endpoint id",
		); err != nil {
			p.logger.Warn("failed to write error response", zap.Error(err))
		}
		return
	}

	logger = logger.With(zap.String("endpoint-id", endpointID))

	r.Header.Add("x-piko-forward", "true")

	// If there is a connected upstream, attempt to forward the request to one
	// of those upstreams. Note this includes remote nodes that are reporting
	// they have an available upstream. We don't allow multiple hops, so if
	// forwarded is true we only select from local nodes.
	upstream, ok := p.upstreams.Select(endpointID, !forwarded)
	if ok {
		p.reverseProxyUpstream(w, r, upstream, logger)
		return
	}

	if err := errorResponse(
		w, http.StatusBadGateway, "no available upstreams",
	); err != nil {
		p.logger.Warn("failed to write error response", zap.Error(err))
	}
}

func (p *ReverseProxy) reverseProxyUpstream(
	w http.ResponseWriter,
	r *http.Request,
	upstream upstream.Upstream,
	logger log.Logger,
) {
	r.URL.Scheme = "http"
	r.URL.Host = upstream.EndpointID()

	// Add the upstream to the context to pass to 'DialContext'.
	ctx := context.WithValue(r.Context(), upstreamContextKey, upstream)
	r = r.WithContext(ctx)

	resp, err := p.upstreamTransport.RoundTrip(r)
	if err != nil {
		logger.Warn("upstream unreachable", zap.Error(err))
		// TODO(andydunstall): Handle different error types.
		if err := errorResponse(
			w, http.StatusBadGateway, "upstream unreachable",
		); err != nil {
			p.logger.Warn("failed to write error response", zap.Error(err))
		}
		return
	}

	// Write the response status, headers and body.
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Warn("failed to write response", zap.Error(err))
		return
	}
}

func (p *ReverseProxy) dialUpstream(ctx context.Context, _, _ string) (net.Conn, error) {
	// As a bit of a hack to work with http.Transport, we add the upstream
	// to the dial context.
	upstream := ctx.Value(upstreamContextKey).(upstream.Upstream)
	return upstream.Dial()
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
