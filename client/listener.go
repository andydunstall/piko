package piko

import "net"

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
