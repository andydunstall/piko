package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/andydunstall/piko/pkg/conn"
	"github.com/gorilla/websocket"
)

// retryableStatusCodes contains a set of HTTP status codes that should be
// retried.
var retryableStatusCodes = map[int]struct{}{
	http.StatusRequestTimeout:      {},
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
}

// RetryableError indicates a error is retryable.
type RetryableError struct {
	err error
}

func NewRetryableError(err error) *RetryableError {
	return &RetryableError{err}
}

func (e *RetryableError) Unwrap() error {
	return e.err
}

func (e *RetryableError) Error() string {
	return e.err.Error()
}

type dialOptions struct {
	token     string
	tlsConfig *tls.Config
}

type DialOption interface {
	apply(*dialOptions)
}

type tokenOption string

func (o tokenOption) apply(opts *dialOptions) {
	opts.token = string(o)
}

func WithToken(token string) DialOption {
	return tokenOption(token)
}

type tlsConfigOption struct {
	TLSConfig *tls.Config
}

func (o tlsConfigOption) apply(opts *dialOptions) {
	opts.tlsConfig = o.TLSConfig
}

func WithTLSConfig(config *tls.Config) DialOption {
	return tlsConfigOption{TLSConfig: config}
}

// Conn implements a [net.Conn] using WebSockets as the underlying transport.
//
// This adds a small amount of overhead compared to using TCP directly, though
// it means the connection can be used with HTTP servers and load balancers.
type Conn struct {
	wsConn *websocket.Conn

	reader io.Reader
}

func New(wsConn *websocket.Conn) *Conn {
	return &Conn{
		wsConn: wsConn,
		reader: nil,
	}
}

func Dial(ctx context.Context, url string, opts ...DialOption) (*Conn, error) {
	options := dialOptions{}
	for _, o := range opts {
		o.apply(&options)
	}

	dialer := &websocket.Dialer{
		HandshakeTimeout: 60 * time.Second,
	}

	header := make(http.Header)
	if options.token != "" {
		header.Set("Authorization", "Bearer "+options.token)
	}

	if options.tlsConfig != nil {
		dialer.TLSClientConfig = options.tlsConfig
	}

	wsConn, resp, err := dialer.DialContext(
		ctx, url, header,
	)
	if err != nil {
		if resp != nil {
			if _, ok := retryableStatusCodes[resp.StatusCode]; ok {
				return nil, conn.NewRetryableError(err)
			}
			return nil, fmt.Errorf("%d: %w", resp.StatusCode, err)
		}
		return nil, conn.NewRetryableError(err)
	}
	return New(wsConn), nil
}

func (c *Conn) Read(b []byte) (int, error) {
	for {
		if c.reader == nil {
			mt, r, err := c.wsConn.NextReader()
			if err != nil {
				return 0, err
			}
			if mt != websocket.BinaryMessage {
				return 0, fmt.Errorf("unexpected message type: %d", mt)
			}
			c.reader = r
		}

		n, err := c.reader.Read(b)
		if n > 0 {
			if err != nil {
				c.reader = nil
				if err == io.EOF {
					err = nil
				}
			}
			return n, err
		}
		if err != io.EOF {
			return 0, err
		}

		// If we get 0 EOF, read from a new reader.
		c.reader = nil
	}
}

func (c *Conn) Write(b []byte) (int, error) {
	if err := c.wsConn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *Conn) Close() error {
	return c.wsConn.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.wsConn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.wsConn.RemoteAddr()
}

func (c *Conn) SetDeadline(t time.Time) error {
	// Note don't just use wsConn.NetConn() as setting deadlines has WebSocket
	// specific logic.
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.wsConn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.wsConn.SetWriteDeadline(t)
}

var _ net.Conn = &Conn{}
