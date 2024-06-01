package piko

import (
	"context"
	"net/url"
	"time"

	"github.com/andydunstall/piko/pkg/websocket"
	"golang.ngrok.com/muxado/v2"
)

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
	sess *muxado.Heartbeat
}

// Connect establishes a new outbound connection with the Piko server. This
//
// This will block until the client can connect.
func Connect(ctx context.Context, opts ...ConnectOption) (*Piko, error) {
	options := connectOptions{
		token: "",
		url:   "ws://localhost:8001",
	}
	for _, o := range opts {
		o.apply(&options)
	}
	// TODO(andydunstall): Add TLS, retries, auth, ...
	conn, err := websocket.Dial(ctx, serverURL(options.url))
	if err != nil {
		return nil, err
	}
	sess := muxado.NewTypedStreamSession(muxado.Client(conn, &muxado.Config{}))
	heartbeat := muxado.NewHeartbeat(
		sess,
		func(d time.Duration, timeout bool) {},
		muxado.NewHeartbeatConfig(),
	)
	heartbeat.Start()

	return &Piko{
		sess: heartbeat,
	}, nil
}

// Listen registers the endpoint with the given ID and returns a [Listener]
// which accepts incoming connections for that endpoint.
//
// [Listener] is a [net.Listener].
// nolint
func (p *Piko) Listen(ctx context.Context, endpointID string) (Listener, error) {
	p.sess.Wait()
	return nil, nil
}

// Close closes the connection to Piko and any open listeners.
func (p *Piko) Close() error {
	return nil
}

func serverURL(urlStr string) string {
	// Already verified URL in Config.Validate.
	u, _ := url.Parse(urlStr)
	u.Path = "/piko/v1/upstream/ws"
	if u.Scheme == "http" {
		u.Scheme = "ws"
	}
	if u.Scheme == "https" {
		u.Scheme = "wss"
	}
	return u.String()
}
