package websocket

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
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

type options struct {
	token     string
	tlsConfig *tls.Config
}

type Option interface {
	apply(*options)
}

type tokenOption string

func (o tokenOption) apply(opts *options) {
	opts.token = string(o)
}

func WithToken(token string) Option {
	return tokenOption(token)
}

type tlsConfigOption struct {
	TLSConfig *tls.Config
}

func (o tlsConfigOption) apply(opts *options) {
	opts.tlsConfig = o.TLSConfig
}

func WithTLSConfig(config *tls.Config) Option {
	return tlsConfigOption{TLSConfig: config}
}

type Conn struct {
	wsConn *websocket.Conn
}

func NewConn(wsConn *websocket.Conn) *Conn {
	return &Conn{
		wsConn: wsConn,
	}
}

func Dial(ctx context.Context, url string, opts ...Option) (*Conn, error) {
	options := options{}
	for _, o := range opts {
		o.apply(&options)
	}

	dialer := &websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: 45 * time.Second,
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
	return NewConn(wsConn), nil
}

func (c *Conn) ReadMessage() ([]byte, error) {
	mt, message, err := c.wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", mt)
	}
	return message, nil
}

func (c *Conn) NextReader() (io.Reader, error) {
	mt, r, err := c.wsConn.NextReader()
	if err != nil {
		return nil, err
	}
	if mt != websocket.BinaryMessage {
		return nil, fmt.Errorf("unexpected websocket message type: %d", mt)
	}
	return r, nil
}

func (c *Conn) WriteMessage(b []byte) error {
	return c.wsConn.WriteMessage(websocket.BinaryMessage, b)
}

func (c *Conn) NextWriter() (io.WriteCloser, error) {
	return c.wsConn.NextWriter(websocket.BinaryMessage)
}

func (c *Conn) Addr() string {
	return c.wsConn.RemoteAddr().String()
}

func (c *Conn) Close() error {
	return c.wsConn.Close()
}
