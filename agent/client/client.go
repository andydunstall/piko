package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"sync"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/websocket"
	"go.uber.org/zap"
)

const (
	// defaultUpstreamURL is the URL of the Piko upstream port when running
	// locally.
	defaultUpstreamURL = "ws://localhost:8001"

	// defaultProxyURL is the URL of the Piko proxy port when running locally.
	defaultProxyURL = "ws://localhost:8000"
)

// Client manages registering listeners with Piko.
//
// The client establishes an outbound-only connection to the server for each
// listener. Proxied connections for the listener are then multiplexed over
// that outbound connection. Therefore the client never exposes a port.
type Client struct {
	options options
	logger  log.Logger
}

func New(opts ...Option) *Client {
	options := options{
		token:       "",
		upstreamURL: defaultUpstreamURL,
		proxyURL:    defaultProxyURL,
		logger:      log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	return &Client{
		options: options,
		logger:  options.logger,
	}
}

// Listen listens for connections for the given endpoint ID.
//
// Listen will block until the listener has been registered.
//
// The returned [Listener] is a [net.Listener].
func (c *Client) Listen(ctx context.Context, endpointID string) (Listener, error) {
	return listen(ctx, endpointID, c.options, c.logger)
}

// ListenAndForward listens for connections on the given endpoint ID and
// forwards to the configured address.
func (c *Client) ListenAndForward(
	ctx context.Context, endpointID string, addr string,
) error {
	ln, err := listen(ctx, endpointID, c.options, c.logger)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.AcceptWithContext(ctx)
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		go c.forwardConn(ctx, conn, addr)
	}
}

// Dial opens a TCP connection to an upstream listening on the given endpoint
// ID via Piko.
func (c *Client) Dial(ctx context.Context, endpointID string) (net.Conn, error) {
	return websocket.Dial(ctx, proxyTCPURL(c.options.proxyURL, endpointID))
}

func (c *Client) forwardConn(ctx context.Context, conn net.Conn, addr string) {
	defer conn.Close()

	dialer := &net.Dialer{}
	upstream, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		c.logger.Error(
			"failed to dial upstream",
			zap.String("addr", addr),
			zap.Error(err),
		)
		return
	}

	g := &sync.WaitGroup{}
	g.Add(2)
	go func() {
		defer g.Done()
		defer conn.Close()
		// nolint
		io.Copy(conn, upstream)
	}()
	go func() {
		defer g.Done()
		defer upstream.Close()
		// nolint
		io.Copy(upstream, conn)
	}()
	g.Wait()
}

func proxyTCPURL(urlStr, endpointID string) string {
	// Already verified URL in Config.Validate.
	u, _ := url.Parse(urlStr)
	u.Path += "/_piko/v1/tcp/" + endpointID
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String()
}
