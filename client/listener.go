package piko

import (
	"net"

	"golang.ngrok.com/muxado/v2"
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
	sess *muxado.Heartbeat
}

func (l *listener) Accept() (net.Conn, error) {
	stream, err := l.sess.AcceptTypedStream()
	if err != nil {
		return nil, err
	}
	return stream, nil
}

func (l *listener) Close() error {
	return nil
}

func (l *listener) Addr() net.Addr {
	return nil
}

func (l *listener) EndpointID() string {
	return ""
}

var _ Listener = &listener{}
