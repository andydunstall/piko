package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
	"go.uber.org/zap"
)

var (
	errEndpointNotFound = errors.New("not endpoint found")
)

// Proxy is responsible for forwarding requests to upstream endpoints.
type Proxy struct {
	local  *localProxy
	remote *remoteProxy

	logger log.Logger
}

func NewProxy(networkMap *netmap.NetworkMap, opts ...Option) *Proxy {
	options := defaultOptions()
	for _, opt := range opts {
		opt.apply(&options)
	}

	logger := options.logger.WithSubsystem("proxy")
	return &Proxy{
		local:  newLocalProxy(logger),
		remote: newRemoteProxy(networkMap, options.forwarder, logger),
		logger: logger,
	}
}

// Request forwards the given HTTP request to an upstream endpoint and returns
// the response.
//
// If the request fails returns a response with status:
// - Missing endpoint ID: 401 (Bad request)
// - Upstream unreachable: 503 (Service unavailable)
// - Timeout: 504 (Gateway timeout)
func (p *Proxy) Request(
	ctx context.Context,
	r *http.Request,
) *http.Response {
	// Whether the request was forwarded from another Pico node.
	forwarded := r.Header.Get("x-pico-forward") == "true"

	logger := p.logger.With(
		zap.String("host", r.Host),
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
	)

	endpointID := endpointIDFromRequest(r)
	if endpointID == "" {
		logger.Warn("request: missing endpoint id")
		return errorResponse(http.StatusBadRequest, "missing pico endpoint id")
	}

	// Attempt to send to an endpoint connected to the local node.
	resp, err := p.local.Request(ctx, endpointID, r)
	if err == nil {
		return resp
	}
	if !errors.Is(err, errEndpointNotFound) {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("endpoint timeout", zap.Error(err))

			return errorResponse(
				http.StatusGatewayTimeout,
				"endpoint timeout",
			)
		}

		logger.Warn("endpoint unreachable", zap.Error(err))
		return errorResponse(
			http.StatusServiceUnavailable,
			"endpoint unreachable",
		)
	}

	// If the request is from another Pico node though we don't have a
	// connection for the endpoint, we don't forward again but return an
	// error.
	if forwarded {
		logger.Warn("request: endpoint not found")
		return errorResponse(http.StatusServiceUnavailable, "endpoint not found")
	}

	// Set the 'x-pico-forward' before forwarding to a remote node.
	r.Header.Set("x-pico-forward", "true")

	// Attempt to send the request to a Pico node with a connection for the
	// endpoint.
	resp, err = p.remote.Request(ctx, endpointID, r)
	if err == nil {
		return resp
	}
	if !errors.Is(err, errEndpointNotFound) {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Warn("endpoint timeout", zap.Error(err))

			return errorResponse(
				http.StatusGatewayTimeout,
				"endpoint timeout",
			)
		}

		logger.Warn("endpoint unreachable", zap.Error(err))
		return errorResponse(
			http.StatusServiceUnavailable,
			"endpoint unreachable",
		)
	}

	logger.Warn("request: endpoint not found")
	return errorResponse(http.StatusServiceUnavailable, "endpoint not found")
}

// AddConn registers a connection for an endpoint.
func (p *Proxy) AddConn(conn Conn) {
	p.logger.Info(
		"add conn",
		zap.String("endpoint-id", conn.EndpointID()),
		zap.String("addr", conn.Addr()),
	)
	p.local.AddConn(conn)
	p.remote.AddConn(conn)
}

// RemoveConn removes a connection for an endpoint.
func (p *Proxy) RemoveConn(conn Conn) {
	p.logger.Info(
		"remove conn",
		zap.String("endpoint-id", conn.EndpointID()),
		zap.String("addr", conn.Addr()),
	)
	p.local.RemoveConn(conn)
	p.remote.RemoveConn(conn)
}

// endpointIDFromRequest returns the endpoint ID from the HTTP request, or an
// empty string if no endpoint ID is specified.
//
// This will check both the 'x-pico-endpoint' header and 'Host' header, where
// x-pico-endpoint takes precedence.
func endpointIDFromRequest(r *http.Request) string {
	endpointID := r.Header.Get("x-pico-endpoint")
	if endpointID != "" {
		return endpointID
	}

	host := r.Host
	if host != "" && strings.Contains(host, ".") {
		// If a host is given and contains a separator, use the bottom-level
		// domain as the endpoint ID.
		//
		// Such as if the domain is 'xyz.pico.example.com', then 'xyz' is the
		// endpoint ID.
		return strings.Split(host, ".")[0]
	}

	return ""
}

type errorMessage struct {
	Error string `json:"error"`
}

func errorResponse(statusCode int, message string) *http.Response {
	m := &errorMessage{
		Error: message,
	}
	b, _ := json.Marshal(m)
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}
