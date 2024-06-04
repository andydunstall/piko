package piko

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"time"

	"github.com/andydunstall/piko/pkg/backoff"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/mux"
	"github.com/andydunstall/piko/pkg/websocket"
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

	mux   *mux.Session
	muxMu sync.Mutex

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
	mux, err := ln.connect(ctx)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	ln.mux = mux

	return ln, nil
}

// Accept accepts a proxied connection for the endpoint.
func (l *listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.mux.Accept()
		if err == nil {
			return conn, nil
		}

		if l.closeCtx.Err() != nil {
			return nil, err
		}

		l.logger.Warn("failed to accept conn", zap.Error(err))

		mux, err := l.connect(l.closeCtx)
		if err != nil {
			return nil, err
		}

		l.muxMu.Lock()
		l.mux = mux
		l.muxMu.Unlock()
	}
}

func (l *listener) Addr() net.Addr {
	return &pikoAddr{endpointID: l.endpointID}
}

func (l *listener) Close() error {
	l.closeCancel()

	l.muxMu.Lock()
	err := l.mux.Close()
	l.muxMu.Unlock()

	return err
}

func (l *listener) EndpointID() string {
	return l.endpointID
}

func (l *listener) connect(ctx context.Context) (*mux.Session, error) {
	backoff := backoff.New(0, minReconnectBackoff, maxReconnectBackoff)
	for {
		conn, err := websocket.Dial(
			ctx,
			serverURL(l.options.url, l.endpointID),
			websocket.WithToken(l.options.token),
			websocket.WithTLSConfig(l.options.tlsConfig),
		)
		if err == nil {
			l.logger.Debug(
				"listener connected",
				zap.String("url", serverURL(l.options.url, l.endpointID)),
			)

			return mux.OpenClient(conn), nil
		}

		var retryableError *websocket.RetryableError
		if !errors.As(err, &retryableError) {
			l.logger.Error(
				"failed to connect to server; non-retryable",
				zap.String("url", serverURL(l.options.url, l.endpointID)),
				zap.Error(err),
			)
			return nil, err
		}

		l.logger.Warn(
			"failed to connect to server; retrying",
			zap.String("url", serverURL(l.options.url, l.endpointID)),
			zap.Error(err),
		)

		if !backoff.Wait(ctx) {
			return nil, ctx.Err()
		}
	}
}

var _ Listener = &listener{}

func serverURL(urlStr, endpointID string) string {
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
