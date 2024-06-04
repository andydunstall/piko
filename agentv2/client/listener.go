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
	// TODO(andydunstall): Doesn't yet notify the server that the listener is
	// closed. This is ok for now as the client is only used in agent.

	if !l.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(l.acceptCh)
	return nil
}

func (l *listener) Addr() net.Addr {
	// TODO(andydunstall)
	return nil
}

func (l *listener) EndpointID() string {
	return l.endpointID
}

func (l *listener) OnConn(conn net.Conn) {
	l.acceptCh <- conn
}

var _ Listener = &listener{}
