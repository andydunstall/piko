package server

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/pkg/status"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type endpoint struct {
	streams   []*rpc.Stream
	nextIndex int
}

func (e *endpoint) AddUpstream(s *rpc.Stream) {
	e.streams = append(e.streams, s)
}

func (e *endpoint) RemoveUpstream(s *rpc.Stream) bool {
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

func (e *endpoint) Next() *rpc.Stream {
	if len(e.streams) == 0 {
		return nil
	}

	s := e.streams[e.nextIndex]
	e.nextIndex++
	e.nextIndex %= len(e.streams)
	return s
}

type proxy struct {
	endpoints map[string]*endpoint

	mu sync.Mutex

	logger *log.Logger
}

func newProxy(logger *log.Logger) *proxy {
	return &proxy{
		endpoints: make(map[string]*endpoint),
		logger:    logger.WithSubsystem("proxy"),
	}
}

func (p *proxy) Request(ctx context.Context, r *http.Request) (*http.Response, error) {
	endpointID := r.Header.Get("x-pico-endpoint")
	if endpointID == "" {
		p.logger.Warn(
			"failed to proxy request: missing endpoint id",
			zap.String("path", r.URL.Path),
			zap.String("method", r.Method),
		)
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
	return resp, nil
}

func (p *proxy) AddUpstream(endpointID string, stream *rpc.Stream) {
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
}

func (p *proxy) RemoveUpstream(endpointID string, stream *rpc.Stream) {
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
}

func (p *proxy) request(ctx context.Context, endpointID string, r *http.Request) (*http.Response, error) {
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
