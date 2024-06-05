package mux

import (
	"io"
	"net"
	"time"

	"golang.ngrok.com/muxado/v2"
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
			return nil, net.ErrClosed
		}
		return nil, err
	}
	return conn, nil
}

func (s *Session) Close() error {
	return s.mux.Close()
}

func (s *Session) onHeartbeat(_ time.Duration, timeout bool) {
	if timeout {
		s.mux.Close()
	}
}
