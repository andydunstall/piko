package client

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/andydunstall/yamux"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/backoff"
	"github.com/andydunstall/piko/pkg/websocket"
)

var (
	ErrClosed = errors.New("closed")
)

// Upstream manages listening on upstream endpoints.
//
// The client establishes an outbound-only connection to the Piko server for
// each listener. Connections to the listener are then multiplexed via this
// outbound-connection. This means the client can treat incoming connections
// as normal connections using [net.Listener], without exposing a port.
//
// Listens recover from any transient networking issues by reconnecting to
// the Piko server.
type Upstream struct {
	// URL is the URL of the Piko server to connect to.
	//
	// This must be the URL of the 'upstream' port.
	// Defaults to 'http://localhost:8001'.
	URL *url.URL

	// Token configures the API key token to authenticate the listener with the
	// Piko server.
	//
	// Defaults to no authentication.
	Token string

	// TLSConfig specifies the TLS configuration to use with the Piko server.
	//
	// If nil, the default configuration is used.
	TLSConfig *tls.Config

	// MinReconnectBackoff is the minimum backoff when reconnecting.
	//
	// Defaults to 100ms.
	MinReconnectBackoff time.Duration

	// MaxReconnectBackoff is the maximum backoff when reconnecting.
	//
	// Defaults to 15s.
	MaxReconnectBackoff time.Duration

	// Logger is an optional logger to log connection state changes.
	Logger Logger
}

// Listen listens for connections on the given endpoint.
func (u *Upstream) Listen(ctx context.Context, endpointID string) (Listener, error) {
	ln := newListener(endpointID, u, u.logger())
	if err := ln.connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return ln, nil
}

// ListenAndForward listens for connections on the given endpoint and
// forwards them to the upstream address.
//
// This synchronously connects to the Piko server and listens on the endpoint,
// then starts a background 'forwarder'. The forwarder is returned and will
// run in the background until either the context is canclled or the returned
// forwarder is closed.
func (u *Upstream) ListenAndForward(
	ctx context.Context, endpointID string, addr string,
) (*Forwarder, error) {
	ln := newListener(endpointID, u, u.logger())
	if err := ln.connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return newForwarder(ctx, ln, addr, u.logger()), nil
}

func (u *Upstream) connect(ctx context.Context, endpointID string) (*yamux.Session, error) {
	minReconnectBackoff := u.MinReconnectBackoff
	if minReconnectBackoff == 0 {
		minReconnectBackoff = time.Millisecond * 100
	}
	maxReconnectBackoff := u.MaxReconnectBackoff
	if maxReconnectBackoff == 0 {
		maxReconnectBackoff = time.Second * 15
	}

	backoff := backoff.New(0, minReconnectBackoff, maxReconnectBackoff)
	for {
		url := u.listenURL(endpointID)

		u.logger().Debug(
			"connecting",
			zap.String("endpoint-id", endpointID),
			zap.String("url", url),
		)

		conn, err := websocket.Dial(
			ctx,
			url,
			websocket.WithToken(u.Token),
			websocket.WithTLSConfig(u.TLSConfig),
		)
		if err == nil {
			u.logger().Debug(
				"connected",
				zap.String("endpoint-id", endpointID),
				zap.String("url", url),
			)

			muxConfig := yamux.DefaultConfig()
			muxConfig.Logger = nil
			muxConfig.LogOutput = &yamuxLogWriter{logger: u.logger()}
			sess, err := yamux.Client(conn, muxConfig)
			if err != nil {
				// Will not happen.
				panic("yamux client: " + err.Error())
			}
			return sess, nil
		}

		if ctx.Err() != nil {
			// If cancelled return without logging or retrying.
			return nil, ctx.Err()
		}

		var retryableError *websocket.RetryableError
		if !errors.As(err, &retryableError) {
			u.logger().Error(
				"connect failed; non-retryable",
				zap.String("endpoint-id", endpointID),
				zap.String("url", url),
				zap.Error(err),
			)
			return nil, err
		}

		u.logger().Warn(
			"connect failed; retrying",
			zap.String("endpoint-id", endpointID),
			zap.String("url", url),
			zap.Error(err),
		)

		if !backoff.Wait(ctx) {
			return nil, ctx.Err()
		}
	}
}

func (u *Upstream) listenURL(endpointID string) string {
	var listenURL url.URL
	if u.URL == nil {
		listenURL = url.URL{
			Scheme: "http",
			Host:   "localhost:8001",
		}
	} else {
		listenURL = *u.URL
	}

	// Add the listen path to the URL.
	listenURL.Path += "/piko/v1/upstream/" + endpointID

	// Set the scheme to WebSocket.
	if listenURL.Scheme == "http" {
		listenURL.Scheme = "ws"
	}
	if listenURL.Scheme == "https" {
		listenURL.Scheme = "wss"
	}

	return listenURL.String()
}

func (u *Upstream) logger() Logger {
	if u.Logger == nil {
		return zap.NewNop()
	}
	return u.Logger
}
