package client

import "net"

// Listener is a [net.Listener] that accepts incoming connections for
// Piko endpoints.
type Listener interface {
	net.Listener

	// EndpointID returns the ID of the endpoint this is listening for
	// connections on.
	EndpointID() string
}

type listener struct {
}

func (l *listener) Accept() (net.Conn, error) {
	return nil, nil
}

func (l *listener) Addr() net.Addr {
	return nil
}

func (l *listener) Close() error {
	return nil
}

func (l *listener) EndpointID() string {
	return ""
}

var _ Listener = &listener{}
