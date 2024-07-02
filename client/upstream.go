package client

import (
	"context"
	"crypto/tls"
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
	// This must be the URL of the 'upstream' port. Defaults to
	// 'http://localhost:8001'.
	URL string

	// Token configures the API key token to authenticate the listener with the
	// Piko server.
	Token string

	// TLSConfig specifies the TLS configuration to use with the Piko server.
	//
	// If nil, the default configuration is used.
	TLSConfig *tls.Config

	// Logger is used to log connection state changes.
	Logger Logger
}

// Listen listens for connections on the given endpoint.
func (u *Upstream) Listen(_ context.Context, _ string) (Listener, error) {
	return nil, nil
}

// ListenAndForward listens for connections on the given endpoint and
// forwards them to the given address.
//
// This will block until the context is canceled.
func (u *Upstream) ListenAndForward(_ context.Context, _ string, _ string) error {
	return nil
}
