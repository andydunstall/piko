package client

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"

	"github.com/andydunstall/piko/pkg/websocket"
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
	URL *url.URL

	// TLSConfig specifies the TLS configuration to use with the Piko server.
	//
	// If nil, the default configuration is used.
	TLSConfig *tls.Config
}

// Dial opens a TCP connection to the endpoint with the given ID and
// returns a [net.Conn].
func (d *Dialer) Dial(ctx context.Context, endpointID string) (net.Conn, error) {
	// Dialing is simply opening a WebSocket connection to the target endpoint,
	// then wrapping the WebSocket in a net.Conn.
	return websocket.Dial(ctx, d.dialURL(endpointID))
}

func (d *Dialer) dialURL(endpointID string) string {
	var dialURL url.URL
	if d.URL == nil {
		dialURL = url.URL{
			Scheme: "http",
			Host:   "localhost:8000",
		}
	} else {
		dialURL = *d.URL
	}

	// Add the dial path to the URL.
	dialURL.Path += "/_piko/v1/tcp/" + endpointID

	// Set the scheme to WebSocket.
	if dialURL.Scheme == "http" {
		dialURL.Scheme = "ws"
	}
	if dialURL.Scheme == "https" {
		dialURL.Scheme = "wss"
	}

	return dialURL.String()
}
