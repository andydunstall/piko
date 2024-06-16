package proxy

import (
	"io"
	"net"
	"net/http"
	"sync"

	"github.com/andydunstall/piko/pkg/log"
	pikowebsocket "github.com/andydunstall/piko/pkg/websocket"
	"github.com/andydunstall/piko/server/upstream"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// TCPProxy proxies TCP traffic to upstream listeners.
//
// Incoming TCP traffic is sent over WebSockets by a Piko client, then
// forwarded to an upstream via a multiplexed stream.
type TCPProxy struct {
	upstreams upstream.Manager

	websocketUpgrader *websocket.Upgrader

	logger log.Logger
}

func NewTCPProxy(upstreams upstream.Manager, logger log.Logger) *TCPProxy {
	return &TCPProxy{
		upstreams:         upstreams,
		websocketUpgrader: &websocket.Upgrader{},
		logger:            logger.WithSubsystem("proxy.tcp"),
	}
}

func (p *TCPProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, endpointID string) {
	// If there is a connected upstream, attempt to forward the request to one
	// of those upstreams. Note this includes remote nodes that are reporting
	// they have an available upstream. We don't allow multiple hops, so if
	// forwarded is true we only select from local nodes.
	//
	// TODO(andydunstall): Forwarding to a remote node isn't support for TCP
	// yet.
	upstream, ok := p.upstreams.Select(endpointID, false)
	if !ok {
		_ = errorResponse(w, http.StatusBadGateway, "no available upstreams")
		return
	}

	upstreamConn, err := upstream.Dial()
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

	forward(upstreamConn, downstreamConn)
}

func forward(conn1 net.Conn, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defer conn1.Close()
		// nolint
		io.Copy(conn1, conn2)
	}()
	go func() {
		defer wg.Done()
		defer conn2.Close()
		// nolint
		io.Copy(conn2, conn1)
	}()
	wg.Wait()
}
