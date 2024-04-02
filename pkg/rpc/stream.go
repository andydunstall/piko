package rpc

import (
	"context"

	"github.com/andydunstall/pico/pkg/conn"
)

// Stream represents a bi-directional RPC stream between two peers. Either peer
// can send an RPC request to the other.
//
// The stream uses the underlying bi-directional connection to send RPC
// requests, and multiplexes multiple concurrent request/response RPCs on the
// same connection.
//
// Incoming RPC requests are handled in their own goroutine to avoid blocking
// the stream.
type Stream struct {
}

func NewStream(conn conn.Conn) *Stream {
	return &Stream{}
}

// RPC sends the given request message to the peer and returns the response or
// an error.
//
// RPC is thread safe.
func (s *Stream) RPC(ctx context.Context, rpcType Type, req []byte) ([]byte, error) {
	return nil, nil
}

func (s *Stream) Close() error {
	return nil
}
