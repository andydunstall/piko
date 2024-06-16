package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/andydunstall/piko/pkg/backoff"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/websocket"
	"github.com/hashicorp/yamux"
	"go.uber.org/zap"
)

const (
	minReconnectBackoff = time.Millisecond * 100
	maxReconnectBackoff = time.Second * 15
)

type pikoAddr struct {
	endpointID string
}

func (a *pikoAddr) Network() string {
	return "tcp"
}

func (a *pikoAddr) String() string {
	return a.endpointID
}

// Listener is a [net.Listener] that accepts incoming connections for endpoints
// registered with the server by the client.
//
// Closing the listener will unregister for the endpoint.
type Listener interface {
	net.Listener

	// EndpointID returns the ID of the endpoint this is listening for
	// connections on.
	EndpointID() string
}

type listener struct {
	endpointID string

	sess *yamux.Session

	options options

	closeCtx    context.Context
	closeCancel func()

	logger log.Logger
}

func listen(
	ctx context.Context,
	endpointID string,
	options options,
	logger log.Logger,
) (*listener, error) {
	closeCtx, closeCancel := context.WithCancel(context.Background())
	ln := &listener{
		endpointID:  endpointID,
		options:     options,
		closeCtx:    closeCtx,
		closeCancel: closeCancel,
		logger:      logger,
	}
	sess, err := ln.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	ln.sess = sess

	return ln, nil
}

// Accept accepts a proxied connection for the endpoint.
func (l *listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.sess.Accept()
		if err == nil {
			return conn, nil
		}

		if l.closeCtx.Err() != nil {
			return nil, err
		}

		l.logger.Warn("failed to accept conn", zap.Error(err))

		sess, err := l.connect(l.closeCtx)
		if err != nil {
			return nil, err
		}

		l.sess = sess
	}
}

func (l *listener) Addr() net.Addr {
	return &pikoAddr{endpointID: l.endpointID}
}

func (l *listener) Close() error {
	l.closeCancel()

	return l.sess.Close()
}

func (l *listener) EndpointID() string {
	return l.endpointID
}

func (l *listener) connect(ctx context.Context) (*yamux.Session, error) {
	backoff := backoff.New(0, minReconnectBackoff, maxReconnectBackoff)
	for {
		conn, err := websocket.Dial(
			ctx,
			upstreamURL(l.options.upstreamURL, l.endpointID),
			websocket.WithToken(l.options.token),
			websocket.WithTLSConfig(l.options.tlsConfig),
		)
		if err == nil {
			l.logger.Debug(
				"listener connected",
				zap.String("url", upstreamURL(l.options.upstreamURL, l.endpointID)),
			)

			muxConfig := yamux.DefaultConfig()
			muxConfig.Logger = l.logger.StdLogger(zap.WarnLevel)
			muxConfig.LogOutput = nil
			sess, err := yamux.Client(conn, muxConfig)
			if err != nil {
				// Will not happen.
				panic("yamux client: " + err.Error())
			}
			return sess, nil
		}

		var retryableError *websocket.RetryableError
		if !errors.As(err, &retryableError) {
			l.logger.Error(
				"failed to connect to server; non-retryable",
				zap.String("url", upstreamURL(l.options.upstreamURL, l.endpointID)),
				zap.Error(err),
			)
			return nil, err
		}

		l.logger.Warn(
			"failed to connect to server; retrying",
			zap.String("url", upstreamURL(l.options.upstreamURL, l.endpointID)),
			zap.Error(err),
		)

		if !backoff.Wait(ctx) {
			return nil, ctx.Err()
		}
	}
}

var _ Listener = &listener{}

func upstreamURL(urlStr, endpointID string) string {
	// Already verified URL in Config.Validate.
	u, _ := url.Parse(urlStr)
	u.Path += "/piko/v1/upstream/" + endpointID
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String()
}
