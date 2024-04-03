package server

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/andydunstall/pico/api"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/pkg/rpc"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

var (
	errNoHealthyUpstream   = errors.New("no healthy upstream")
	errUpstreamTimeout     = errors.New("upstream timeout")
	errUpstreamUnreachable = errors.New("upstream unreachable")
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
	p.mu.Lock()

	endpointID := r.Header.Get("x-pico-endpoint")
	if endpointID == "" {
		p.logger.Warn(
			"request; missing endpoint id",
			zap.String("path", r.URL.Path),
		)

		p.mu.Unlock()
		return nil, errNoHealthyUpstream
	}

	endpoint, ok := p.endpoints[endpointID]
	if !ok {
		p.mu.Unlock()

		p.logger.Warn(
			"request; endpoint not found",
			zap.String("endpoint-id", endpointID),
			zap.String("path", r.URL.Path),
		)

		return nil, errNoHealthyUpstream
	}
	stream := endpoint.Next()
	p.mu.Unlock()

	// Write the HTTP request to a buffer.
	var buffer bytes.Buffer
	if err := r.Write(&buffer); err != nil {
		return nil, err
	}

	protoReq := &api.ProxyHttpReq{
		HttpReq: buffer.Bytes(),
	}
	payload, err := proto.Marshal(protoReq)
	if err != nil {
		return nil, err
	}
	b, err := stream.RPC(ctx, rpc.TypeProxyHTTP, payload)
	if err != nil {
		return nil, err
	}

	var protoResp api.ProxyHttpResp
	if err := proto.Unmarshal(b, &protoResp); err != nil {
		return nil, err
	}

	if protoResp.Error != nil && protoResp.Error.Status != api.ProxyHttpStatus_OK {
		switch protoResp.Error.Status {
		case api.ProxyHttpStatus_UPSTREAM_TIMEOUT:
			return nil, errUpstreamTimeout
		case api.ProxyHttpStatus_UPSTREAM_UNREACHABLE:
			return nil, errUpstreamUnreachable
		default:
			return nil, fmt.Errorf(protoResp.Error.Message)
		}
	}

	httpResp, err := http.ReadResponse(
		bufio.NewReader(bytes.NewReader(protoResp.HttpResp)), r,
	)
	if err != nil {
		return nil, err
	}
	return httpResp, nil
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

	p.logger.Warn(
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

	p.logger.Warn(
		"removed upstream",
		zap.String("endpoint-id", endpointID),
	)
}
