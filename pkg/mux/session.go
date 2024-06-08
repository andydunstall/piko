package mux

import (
	"io"
	"net"

	"golang.ngrok.com/muxado/v2"
)

// Session is a connection between two nodes that multiplexes multiple
// connections on the underlying connection.
//
// Session is a wrapper for the 'muxado' library.
type Session struct {
	mux muxado.Session
}

func OpenClient(conn io.ReadWriteCloser) *Session {
	return &Session{
		mux: muxado.Client(conn, &muxado.Config{}),
	}
}

func OpenServer(conn io.ReadWriteCloser) *Session {
	return &Session{
		mux: muxado.Server(conn, &muxado.Config{}),
	}
}

func (s *Session) Dial() (net.Conn, error) {
	return s.mux.OpenStream()
}

// Accept accepts a multiplexed connection.
func (s *Session) Accept() (net.Conn, error) {
	conn, err := s.mux.AcceptStream()
	if err != nil {
		muxadoErr, _ := muxado.GetError(err)
		if muxadoErr == muxado.SessionClosed {
			return nil, net.ErrClosed
		}
		return nil, err
	}
	return conn, nil
}

func (s *Session) Wait() error {
	err, _, _ := s.mux.Wait()
	return err
}

func (s *Session) Close() error {
	return s.mux.Close()
}
