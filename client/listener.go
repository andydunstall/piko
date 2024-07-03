package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"

	"github.com/hashicorp/yamux"
	"go.uber.org/zap"
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

// Listener is a [net.Listener] that accepts incoming connections for
// Piko endpoints.
type Listener interface {
	net.Listener

	// EndpointID returns the ID of the endpoint this is listening for
	// connections on.
	EndpointID() string
}

type listener struct {
	endpointID string

	upstream *Upstream

	// sess contains the connected yamux session to the Piko server.
	//
	// This is used to accept incoming multiplexed connections.
	sess *yamux.Session

	// closeCtx closes the listener on listener.Close()
	closeCtx    context.Context
	closeCancel context.CancelFunc

	logger Logger
}

func newListener(endpointID string, upstream *Upstream, logger Logger) *listener {
	closeCtx, closeCancel := context.WithCancel(context.Background())
	return &listener{
		endpointID:  endpointID,
		upstream:    upstream,
		closeCtx:    closeCtx,
		closeCancel: closeCancel,
		logger:      logger,
	}
}

func (l *listener) Accept() (net.Conn, error) {
	for {
		conn, err := l.sess.Accept()
		if err == nil {
			return conn, nil
		}

		if l.closeCtx.Err() != nil {
			// If the listener was closed then returnreturn
			return nil, err
		}

		if err != io.EOF && !strings.Contains(err.Error(), "closed") && !strings.Contains(err.Error(), "reset by peer") {
			l.logger.Error("failed to accept conn", zap.Error(err))
		} else {
			l.logger.Warn("disconnected; reconnecting")
		}

		if err := l.connect(l.closeCtx); err != nil {
			return nil, fmt.Errorf("connect: %w", err)
		}
	}
}

func (l *listener) Addr() net.Addr {
	return &pikoAddr{endpointID: l.endpointID}
}

func (l *listener) Close() error {
	// Cancel to stop reconnect attempts.
	l.closeCancel()
	// Close the current session.
	return l.sess.Close()
}

func (l *listener) EndpointID() string {
	return l.endpointID
}

// connect to Piko for the listener endpoint.
//
// The endpoint ID and token are included in the initial request.
func (l *listener) connect(ctx context.Context) error {
	sess, err := l.upstream.connect(ctx, l.endpointID)
	if err != nil {
		return err
	}
	l.sess = sess
	return nil
}

var _ Listener = &listener{}
