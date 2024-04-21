package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/pkg/status"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type localEndpoint struct {
	streams   []*rpc.Stream
	nextIndex int
}

func (e *localEndpoint) AddUpstream(s *rpc.Stream) {
	e.streams = append(e.streams, s)
}

func (e *localEndpoint) RemoveUpstream(s *rpc.Stream) bool {
	for i := 0; i != len(e.streams); i++ {
		if e.streams[i] == s {
			e.streams = append(e.streams[:i], e.streams[i+1:]...)
			if len(e.streams) == 0 {
				return true
			}
			e.nextIndex %= len(e.streams)
			return false
		}
	}
	return len(e.streams) == 0
}

func (e *localEndpoint) Next() *rpc.Stream {
	if len(e.streams) == 0 {
		return nil
	}

	s := e.streams[e.nextIndex]
	e.nextIndex++
	e.nextIndex %= len(e.streams)
	return s
}

// Proxy is responsible for forwarding requests to upstream listeners.
type Proxy struct {
	localEndpoints map[string]*localEndpoint

	mu sync.Mutex

	client *http.Client

	networkMap *netmap.NetworkMap

	metrics *metrics
	logger  *log.Logger
}

func NewProxy(
	networkMap *netmap.NetworkMap,
	registry *prometheus.Registry,
	logger *log.Logger,
) *Proxy {
	metrics := newMetrics()
	if registry != nil {
		metrics.Register(registry)
	}
	return &Proxy{
		localEndpoints: make(map[string]*localEndpoint),
		networkMap:     networkMap,
		client:         &http.Client{},
		metrics:        metrics,
		logger:         logger.WithSubsystem("proxy"),
	}
}

func (p *Proxy) Request(ctx context.Context, r *http.Request) (*http.Response, error) {
	endpointID := parseEndpointID(r)
	if endpointID == "" {
		p.logger.Warn(
			"failed to proxy request: missing endpoint id",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
		)
		p.metrics.ErrorsTotal.Inc()
		return nil, &status.ErrorInfo{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "missing endpoint id",
		}
	}

	start := time.Now()
	resp, err := p.request(ctx, endpointID, r)
	if err != nil {
		p.logger.Warn(
			"failed to proxy request",
			zap.String("endpoint-id", endpointID),
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
			zap.Error(err),
		)
		p.metrics.ErrorsTotal.Inc()
		return nil, err
	}

	p.logger.Debug(
		"proxied request",
		zap.String("endpoint-id", endpointID),
		zap.String("path", r.URL.Path),
		zap.String("method", r.Method),
		zap.Int("status", resp.StatusCode),
		zap.Duration("latency", time.Since(start)),
	)

	p.metrics.RequestsTotal.With(prometheus.Labels{
		"status": strconv.Itoa(resp.StatusCode),
	}).Inc()
	p.metrics.RequestLatency.With(prometheus.Labels{
		"status": strconv.Itoa(resp.StatusCode),
	}).Observe(float64(time.Since(start).Milliseconds()) / 1000)

	return resp, nil
}

func (p *Proxy) AddUpstream(endpointID string, stream *rpc.Stream) {
	p.networkMap.AddLocalEndpoint(endpointID)

	p.mu.Lock()
	defer p.mu.Unlock()

	e, ok := p.localEndpoints[endpointID]
	if !ok {
		e = &localEndpoint{}
	}

	e.AddUpstream(stream)
	p.localEndpoints[endpointID] = e

	p.logger.Info(
		"added upstream",
		zap.String("endpoint-id", endpointID),
	)

	p.metrics.Listeners.Inc()
}

func (p *Proxy) RemoveUpstream(endpointID string, stream *rpc.Stream) {
	p.networkMap.RemoveLocalEndpoint(endpointID)

	p.mu.Lock()
	defer p.mu.Unlock()

	endpoint, ok := p.localEndpoints[endpointID]
	if !ok {
		return
	}
	endpoint.RemoveUpstream(stream)

	p.logger.Info(
		"removed upstream",
		zap.String("endpoint-id", endpointID),
	)

	p.metrics.Listeners.Dec()
}

func (p *Proxy) request(
	ctx context.Context,
	endpointID string,
	r *http.Request,
) (*http.Response, error) {
	listenerStream, ok := p.lookupLocalListener(endpointID)
	if ok {
		return p.requestLocal(ctx, listenerStream, r)
	}

	node, ok := p.networkMap.LookupEndpoint(endpointID)
	if ok {
		return p.requestRemote(ctx, node.ProxyAddr, r)
	}

	return nil, &status.ErrorInfo{
		StatusCode: http.StatusServiceUnavailable,
		Message:    "no upstream found",
	}
}

// lookupLocalListener looks up an RPC stream for an upstream listener for this
// endpoint.
func (p *Proxy) lookupLocalListener(endpointID string) (*rpc.Stream, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoint, ok := p.localEndpoints[endpointID]
	if !ok {
		return nil, false
	}
	stream := endpoint.Next()
	return stream, true
}

func (p *Proxy) requestLocal(
	ctx context.Context,
	stream *rpc.Stream,
	r *http.Request,
) (*http.Response, error) {
	// Write the HTTP request to a buffer.
	var buffer bytes.Buffer
	if err := r.Write(&buffer); err != nil {
		return nil, fmt.Errorf("encode http request: %w", err)
	}

	protoReq := &api.ProxyHttpReq{
		HttpReq: buffer.Bytes(),
	}
	payload, err := proto.Marshal(protoReq)
	if err != nil {
		return nil, fmt.Errorf("encode proto request: %w", err)
	}
	b, err := stream.RPC(ctx, rpc.TypeProxyHTTP, payload)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &status.ErrorInfo{
				StatusCode: http.StatusGatewayTimeout,
				Message:    "upstream timeout",
			}
		}

		return nil, &status.ErrorInfo{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "upstream unreachable",
		}
	}

	var protoResp api.ProxyHttpResp
	if err := proto.Unmarshal(b, &protoResp); err != nil {
		return nil, fmt.Errorf("decode proto response: %w", err)
	}

	if protoResp.Error != nil && protoResp.Error.Status != api.ProxyHttpStatus_OK {
		switch protoResp.Error.Status {
		case api.ProxyHttpStatus_UPSTREAM_TIMEOUT:
			return nil, &status.ErrorInfo{
				StatusCode: http.StatusGatewayTimeout,
				Message:    "upstream timeout",
			}
		case api.ProxyHttpStatus_UPSTREAM_UNREACHABLE:
			return nil, &status.ErrorInfo{
				StatusCode: http.StatusGatewayTimeout,
				Message:    "upstream unreachable",
			}
		default:
			return nil, fmt.Errorf("upstream: %s", protoResp.Error.Message)
		}
	}

	httpResp, err := http.ReadResponse(
		bufio.NewReader(bytes.NewReader(protoResp.HttpResp)), r,
	)
	if err != nil {
		return nil, fmt.Errorf("decode http response: %w", err)
	}
	return httpResp, nil
}

func (p *Proxy) requestRemote(
	ctx context.Context,
	addr string,
	req *http.Request,
) (*http.Response, error) {
	// TODO(andydunstall): Need to limit the number of hops.

	req = req.WithContext(ctx)

	req.URL.Scheme = "http"
	req.URL.Host = addr
	req.RequestURI = ""

	resp, err := p.client.Do(req)
	if err != nil {
		p.logger.Warn(
			"failed to forward request",
			zap.String("method", req.Method),
			zap.String("host", req.URL.Host),
			zap.String("path", req.URL.Path),
			zap.Error(err),
		)

		if errors.Is(err, context.DeadlineExceeded) {
			return nil, &status.ErrorInfo{
				StatusCode: http.StatusGatewayTimeout,
				Message:    "upstream timeout",
			}
		}
		return nil, &status.ErrorInfo{
			StatusCode: http.StatusGatewayTimeout,
			Message:    "upstream unreachable",
		}
	}

	// TODO(andydunstall): Add metrics and extend logging.

	p.logger.Debug(
		"forward",
		zap.String("method", req.Method),
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Int("status", resp.StatusCode),
	)

	return resp, nil
}

func parseEndpointID(r *http.Request) string {
	endpointID := r.Header.Get("x-pico-endpoint")
	if endpointID != "" {
		return endpointID
	}

	host := r.Header.Get("host")
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
