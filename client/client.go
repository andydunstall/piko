package piko

import "context"

// Piko manages registering endpoints with the server and listening for
// incoming connections to those endpoints.
//
// The client establishes an outbound-only connection to the server, so never
// exposes any ports itself. Connections to the registered endpoints will be
// load balanced by the server to any upstreams registered for the endpoint.
// Those connections will then be forwarded to the upstream via the clients
// outbound-only connection.
//
// NOTE the client is not yet functional.
type Piko struct {
}

// Connect establishes a new outbound connection with the Piko server. This
//
// This will block until the client can connect.
// nolint
func Connect(ctx context.Context, opts ...ConnectOption) (*Piko, error) {
	return nil, nil
}

// Listen registers the endpoint with the given ID and returns a [Listener]
// which accepts incoming connections for that endpoint.
//
// [Listener] is a [net.Listener].
// nolint
func (p *Piko) Listen(ctx context.Context, endpointID string) (Listener, error) {
	return nil, nil
}

// Close closes the connection to Piko and any open listeners.
func (p *Piko) Close() error {
	return nil
}
