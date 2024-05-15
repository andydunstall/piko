package proxy

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/andydunstall/piko/pkg/rpc"
)

// Conn represents a connection to an upstream endpoint.
type Conn interface {
	EndpointID() string
	Addr() string
	Request(ctx context.Context, r *http.Request) (*http.Response, error)
}

// RPCConn represents a connection to an upstream endpoint using
// rpc.Stream to exchange messages.
type RPCConn struct {
	endpointID string
	stream     rpc.Stream
}

func NewRPCConn(endpointID string, stream rpc.Stream) *RPCConn {
	return &RPCConn{
		endpointID: endpointID,
		stream:     stream,
	}
}

func (c *RPCConn) EndpointID() string {
	return c.endpointID
}

func (c *RPCConn) Addr() string {
	return c.stream.Addr()
}

func (c *RPCConn) Request(
	ctx context.Context,
	r *http.Request,
) (*http.Response, error) {
	// Encode the HTTP request.
	var buffer bytes.Buffer
	if err := r.Write(&buffer); err != nil {
		return nil, fmt.Errorf("encode http request: %w", err)
	}

	// Forward the request via RPC.
	b, err := c.stream.RPC(ctx, rpc.TypeProxyHTTP, buffer.Bytes())
	if err != nil {
		return nil, fmt.Errorf("rpc: %w", err)
	}

	httpResp, err := http.ReadResponse(
		bufio.NewReader(bytes.NewReader(b)), r,
	)
	if err != nil {
		return nil, fmt.Errorf("decode http response: %w", err)
	}

	return httpResp, nil
}

var _ Conn = &RPCConn{}
