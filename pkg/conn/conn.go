package conn

import "io"

// RetryableError indicates a error is retryable.
type RetryableError struct {
	err error
}

func (e *RetryableError) Unwrap() error {
	return e.err
}

func (e *RetryableError) Error() string {
	return e.err.Error()
}

// Conn represents a bi-directional message-oriented connection between
// two peers.
type Conn interface {
	ReadMessage() ([]byte, error)
	NextReader() (io.Reader, error)
	WriteMessage(b []byte) error
	NextWriter() (io.WriteCloser, error)
	Addr() string
	Close() error
}
