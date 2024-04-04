package proxy

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/pkg/status"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// Proxy is responsible for forwarding requests to upstream listeners.
type Proxy struct {
	endpoints map[string]*endpoint

	mu sync.Mutex

	metrics *metrics
	logger  *log.Logger
}

func NewProxy(registry *prometheus.Registry, logger *log.Logger) *Proxy {
	metrics := newMetrics()
	if registry != nil {
		metrics.Register(registry)
	}
	return &Proxy{
		endpoints: make(map[string]*endpoint),
		metrics:   metrics,
		logger:    logger.WithSubsystem("proxy"),
	}
}

func (p *Proxy) Request(ctx context.Context, r *http.Request) (*http.Response, error) {
	endpointID := r.Header.Get("x-pico-endpoint")
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
	p.mu.Lock()
	defer p.mu.Unlock()

	e, ok := p.endpoints[endpointID]
	if !ok {
		e = &endpoint{}
	}

	e.AddUpstream(stream)
	p.endpoints[endpointID] = e

	p.logger.Info(
		"added upstream",
		zap.String("endpoint-id", endpointID),
	)

	p.metrics.Listeners.Inc()
}

func (p *Proxy) RemoveUpstream(endpointID string, stream *rpc.Stream) {
	p.mu.Lock()
	defer p.mu.Unlock()

	endpoint, ok := p.endpoints[endpointID]
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

func (p *Proxy) request(ctx context.Context, endpointID string, r *http.Request) (*http.Response, error) {
	p.mu.Lock()
	endpoint, ok := p.endpoints[endpointID]
	if !ok {
		p.mu.Unlock()
		return nil, &status.ErrorInfo{
			StatusCode: http.StatusServiceUnavailable,
			Message:    "no upstream found",
		}
	}
	stream := endpoint.Next()
	p.mu.Unlock()

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
