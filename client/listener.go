package piko

import (
	"fmt"
	"net"

	"go.uber.org/atomic"
)

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

	acceptCh chan net.Conn

	closed *atomic.Bool
}

func newListener(endpointID string) *listener {
	return &listener{
		endpointID: endpointID,
		acceptCh:   make(chan net.Conn),
		closed:     atomic.NewBool(false),
	}
}

func (l *listener) Accept() (net.Conn, error) {
	conn, ok := <-l.acceptCh
	if !ok {
		return nil, fmt.Errorf("closed")
	}

	return conn, nil
}

func (l *listener) Close() error {
	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(l.acceptCh)
	// TODO(andydunstall): Tell server.
	return nil
}

func (l *listener) Addr() net.Addr {
	return nil
}

func (l *listener) EndpointID() string {
	return l.endpointID
}

var _ Listener = &listener{}
