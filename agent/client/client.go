package client

import (
	"context"

	"github.com/andydunstall/piko/pkg/log"
)

const (
	// defaultUpstreamURL is the URL of the Piko upstream port when running
	// locally.
	defaultUpstreamURL = "ws://localhost:8001"
)

// Client manages registering listeners with Piko.
//
// The client establishes an outbound-only connection to the server for each
// listener. Proxied connections for the listener are then multiplexed over
// that outbound connection. Therefore the client never exposes a port.
type Client struct {
	options options
	logger  log.Logger
}

func New(opts ...Option) *Client {
	options := options{
		token:       "",
		upstreamURL: defaultUpstreamURL,
		logger:      log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	return &Client{
		options: options,
		logger:  options.logger,
	}
}

// Listen listens for connections for the given endpoint ID.
//
// Listen will block until the listener has been registered.
//
// The returned [Listener] is a [net.Listener].
func (c *Client) Listen(ctx context.Context, endpointID string) (Listener, error) {
	return listen(ctx, endpointID, c.options, c.logger)
}
