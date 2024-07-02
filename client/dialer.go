package client

import (
	"context"
	"crypto/tls"
	"net"
)

// Dialer manages opening connections to upstream endpoints.
//
// Note when using HTTP, you can open a TCP direction directly to Piko without
// using a Piko client as the endpoint to connect to is specified in the HTTP
// request. You only need to use [Dialer] when using 'raw' TCP so the client
// can specify which endpoint to connect to.
type Dialer struct {
	// URL is the URL of the Piko server to connect to.
	//
	// This must be the URL of the 'proxy' port. Defaults to
	// 'http://localhost:8000'.
	URL string

	// TLSConfig specifies the TLS configuration to use with the Piko server.
	//
	// If nil, the default configuration is used.
	TLSConfig *tls.Config
}

// Dial opens a TCP connection to the endpoint with the given ID and
// returns a [net.Conn].
func (d *Dialer) Dial(_ context.Context, _ string) (net.Conn, error) {
	return nil, nil
}
