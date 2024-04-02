package conn

import "io"

// Conn represents a bi-directional message-oriented connection between
// two peers.
type Conn interface {
	ReadMessage() ([]byte, error)
	NextReader() (io.Reader, error)
	WriteMessage(b []byte) error
	NextWriter() (io.WriteCloser, error)
	Close() error
}
