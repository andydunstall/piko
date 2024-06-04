package mux

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/andydunstall/piko/pkg/protocol"
	"golang.ngrok.com/muxado/v2"
)

var (
	ErrSessionClosed = errors.New("session closed")
)

// Session is a connection between two nodes that multiplexes multiple
// connections on the underlying connection.
//
// The session also has heartbeats to verify the underlying connection is
// healthy.
//
// Session is a wrapper for the 'muxado' library.
type Session struct {
	mux *muxado.Heartbeat
}

func OpenClient(conn io.ReadWriteCloser) *Session {
	sess := &Session{}

	mux := muxado.NewHeartbeat(
		muxado.NewTypedStreamSession(
			muxado.Client(conn, &muxado.Config{}),
		),
		sess.onHeartbeat,
		muxado.NewHeartbeatConfig(),
	)
	mux.Start()

	sess.mux = mux

	return sess
}

// Accept accepts a multiplexed connection.
func (s *Session) Accept() (net.Conn, error) {
	conn, err := s.mux.AcceptTypedStream()
	if err != nil {
		muxadoErr, _ := muxado.GetError(err)
		if muxadoErr == muxado.SessionClosed {
			return nil, ErrSessionClosed
		}
		return nil, err
	}
	return conn, nil
}

// RPC sends a JSON RPC request over the underlying connection and returns the
// response.
//
// Each RPC has its own multiplexed connection with a single request and
// response.
func (s *Session) RPC(rpcType protocol.RPCType, req any, resp any) error {
	stream, err := s.mux.OpenTypedStream(muxado.StreamType(rpcType))
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	if err := json.NewEncoder(stream).Encode(req); err != nil {
		return fmt.Errorf("encode req: %w", err)
	}

	if err := json.NewDecoder(stream).Decode(&resp); err != nil {
		return fmt.Errorf("decode resp: %w", err)
	}

	return nil
}

func (s *Session) Close() error {
	return s.mux.Close()
}

func (s *Session) onHeartbeat(_ time.Duration, _ bool) {
}
