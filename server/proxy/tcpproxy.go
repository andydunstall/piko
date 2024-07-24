package proxy

import (
	"errors"
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pkg/log"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
	"github.com/andydunstall/piko/server/upstream"
)

// TCPProxy proxies TCP traffic to upstream listeners.
//
// Incoming TCP traffic is sent over WebSockets by a Piko client, then
// forwarded to an upstream via a multiplexed stream.
type TCPProxy struct {
	upstreams upstream.Manager

	httpProxy *HTTPProxy

	websocketUpgrader *websocket.Upgrader

	logger log.Logger
}

func NewTCPProxy(
	upstreams upstream.Manager,
	httpProxy *HTTPProxy,
	logger log.Logger,
) *TCPProxy {
	return &TCPProxy{
		upstreams:         upstreams,
		httpProxy:         httpProxy,
		websocketUpgrader: &websocket.Upgrader{},
		logger:            logger.WithSubsystem("proxy.tcp"),
	}
}

func (p *TCPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, endpointID string) {
	forwarded := r.Header.Get("x-piko-forward") == "true"

	// If there is a connected upstream, attempt to forward the request to one
	// of those upstreams. Note this includes remote nodes that are reporting
	// they have an available upstream. We don't allow multiple hops, so if
	// forwarded is true we only select from local nodes.
	u, ok := p.upstreams.Select(endpointID, !forwarded)
	if !ok {
		p.logger.Warn(
			"no available upstreams",
			zap.String("endpoint-id", endpointID),
		)

		_ = errorResponse(w, http.StatusBadGateway, "no available upstreams")
		return
	}

	// If the upstream is a remote node rather than a client listener, forward
	// the connection via the HTTP reverse proxy. As it is a WebSocket
	// connection the remote node can handle the connection and forward to an
	// upstream listener.
	if u.Forward() {
		p.httpProxy.ServeHTTPWithUpstream(w, r, endpointID, u)
		return
	}

	upstreamConn, err := u.Dial()
	if err != nil {
		_ = errorResponse(w, http.StatusBadGateway, "upstream unreachable")
		return
	}
	defer upstreamConn.Close()

	wsConn, err := p.websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrade replies to the client so nothing else to do.
		p.logger.Warn("failed to upgrade websocket", zap.Error(err))
		return
	}
	downstreamConn := pikowebsocket.New(wsConn)
	defer downstreamConn.Close()

	p.forward(upstreamConn, downstreamConn)
}

func (p *TCPProxy) forward(upstream net.Conn, downstream net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer upstream.Close()
		_, err := io.Copy(upstream, downstream)
		if err != nil && !errors.Is(err, io.EOF) {
			p.logger.Error("failure to copy from downstream to upstream", zap.Error(err))
		}
	}()
	go func() {
		defer wg.Done()
		defer downstream.Close()
		_, err := io.Copy(downstream, upstream)
		if err != nil && !errors.Is(err, io.EOF) {
			p.logger.Error("failure to copy from upstream to downstream", zap.Error(err))
		}
	}()
	wg.Wait()
}
