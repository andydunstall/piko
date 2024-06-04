package piko

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sync"
	"time"

	"github.com/andydunstall/piko/pkg/backoff"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/pkg/mux"
	"github.com/andydunstall/piko/pkg/protocol"
	"github.com/andydunstall/piko/pkg/websocket"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

const (
	// defaultURL is the URL of the Piko upstream port when running locally.
	defaultURL = "ws://localhost:8001"

	minReconnectBackoff = time.Millisecond * 100
	maxReconnectBackoff = time.Second * 15
)

// Piko manages registering and listening on endpoints.
//
// The client establishes an outbound-only connection to the server, so never
// exposes any ports itself.
//
// The server then load balances incoming connections to any connected
// upstreams for the requested endpoint. These incoming connections are then
// forwarded to the upstream via the upstreams outbound-only connection.
//
// After the initial connection succeeds, the client will reconnect after any
// transient errors.
//
// NOTE the client is still in development...
type Piko struct {
	mux *mux.Session

	listeners   map[string]*listener
	listenersMu sync.Mutex

	closed *atomic.Bool

	logger log.Logger
}

// Connect establishes a new outbound connection with the Piko server.
//
// Block until the client can connect. Returns an error if the context is
// cancelled before the connection can be established, or if the client
// receives a non-retryable error.
func Connect(ctx context.Context, opts ...ConnectOption) (*Piko, error) {
	options := connectOptions{
		token:  "",
		url:    defaultURL,
		logger: log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	client := &Piko{
		listeners: make(map[string]*listener),
		closed:    atomic.NewBool(false),
		logger:    options.logger,
	}
	mux, err := client.connect(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	client.mux = mux

	go client.receive()

	return client, nil
}

// Listen listens for connections for the given endpoint ID.
//
// The returned [Listener] is a [net.Listener].
func (p *Piko) Listen(endpointID string) (Listener, error) {
	ln := newListener(endpointID)
	if !p.addListener(endpointID, ln) {
		return nil, fmt.Errorf("already registered listener for endpoint: %s", endpointID)
	}

	req := &protocol.ListenRequest{
		EndpointID: endpointID,
	}
	var resp protocol.ListenResponse
	if err := p.mux.RPC(protocol.RPCTypeListen, req, &resp); err != nil {
		p.removeListener(endpointID)
		return nil, fmt.Errorf("rpc: %w", err)
	}

	return ln, nil
}

// Close closes the connection to Piko and any open listeners.
func (p *Piko) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	p.listenersMu.Lock()
	for _, ln := range p.listeners {
		ln.Close()
	}
	p.listenersMu.Unlock()
	return p.mux.Close()
}

func (p *Piko) connect(ctx context.Context, options connectOptions) (*mux.Session, error) {
	backoff := backoff.New(
		0,
		minReconnectBackoff,
		maxReconnectBackoff,
	)
	for {
		conn, err := websocket.Dial(
			ctx,
			serverURL(options.url),
			websocket.WithToken(options.token),
			websocket.WithTLSConfig(options.tlsConfig),
		)
		if err == nil {
			p.logger.Debug(
				"connected",
				zap.String("url", serverURL(options.url)),
			)

			return mux.OpenClient(conn), nil
		}

		var retryableError *websocket.RetryableError
		if !errors.As(err, &retryableError) {
			p.logger.Error(
				"failed to connect to server; non-retryable",
				zap.String("url", serverURL(options.url)),
				zap.Error(err),
			)
			return nil, err
		}

		p.logger.Warn(
			"failed to connect to server; retrying",
			zap.String("url", serverURL(options.url)),
			zap.Error(err),
		)

		if !backoff.Wait(ctx) {
			return nil, ctx.Err()
		}
	}
}

func (p *Piko) receive() {
	for {
		conn, err := p.mux.Accept()
		if err != nil {
			if errors.Is(err, mux.ErrSessionClosed) {
				return
			}

			// TODO(andydunstall): Reconnect.

			p.logger.Warn("failed to accept conn", zap.Error(err))
			return
		}

		// Read the proxy header. To avoid the JSON decoder reading the
		// HTTP request itself, the header is prefixed with a size then only
		// that number of bytes may be read.

		var sz int64
		if err := binary.Read(conn, binary.BigEndian, &sz); err != nil {
			p.logger.Warn("failed to read proxy header", zap.Error(err))
			continue
		}

		var header protocol.ProxyHeader
		if err := json.NewDecoder(io.LimitReader(conn, sz)).Decode(&header); err != nil {
			p.logger.Warn("failed to read proxy header", zap.Error(err))
			continue
		}

		ln, ok := p.getListener(header.EndpointID)
		if !ok {
			p.logger.Warn("proxy endpoint not found", zap.Error(err))
			continue
		}

		ln.OnConn(conn)

		p.logger.Debug(
			"accepted conn",
			zap.String("endpoint-id", header.EndpointID),
		)
	}
}

func (p *Piko) getListener(endpointID string) (*listener, bool) {
	p.listenersMu.Lock()
	defer p.listenersMu.Unlock()

	ln, ok := p.listeners[endpointID]
	return ln, ok
}

func (p *Piko) addListener(endpointID string, ln *listener) bool {
	p.listenersMu.Lock()
	defer p.listenersMu.Unlock()

	if _, ok := p.listeners[endpointID]; ok {
		return false
	}
	p.listeners[endpointID] = ln
	return true
}

func (p *Piko) removeListener(endpointID string) {
	p.listenersMu.Lock()
	defer p.listenersMu.Unlock()

	delete(p.listeners, endpointID)
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
