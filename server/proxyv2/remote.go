package proxy

import (
	"context"
	"net/http"

	"github.com/andydunstall/pico/pkg/forwarder"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
)

// remoteProxy is responsible for forwarding requests to Pico server nodes with
// an upstream connection for the target endpoint.
type remoteProxy struct {
	networkMap *netmap.NetworkMap

	forwarder forwarder.Forwarder

	logger log.Logger
}

func newRemoteProxy(
	networkMap *netmap.NetworkMap,
	forwarder forwarder.Forwarder,
	logger log.Logger,
) *remoteProxy {
	return &remoteProxy{
		networkMap: networkMap,
		forwarder:  forwarder,
		logger:     logger,
	}
}

func (p *remoteProxy) Request(
	ctx context.Context,
	endpointID string,
	r *http.Request,
) (*http.Response, error) {
	addr, ok := p.findNode(endpointID)
	if !ok {
		return nil, errEndpointNotFound
	}
	return p.forwarder.Request(ctx, addr, r)
}

func (p *remoteProxy) AddConn(conn Conn) {
	// Update the netmap to notify other nodes that we have a connection for
	// the endpoint.
	p.networkMap.AddLocalEndpoint(conn.EndpointID())
}

func (p *remoteProxy) RemoveConn(conn Conn) {
	p.networkMap.RemoveLocalEndpoint(conn.EndpointID())
}

// findNode looks up a node with an upstream connection for the given endpoint
// and returns the proxy address.
func (p *remoteProxy) findNode(endpointID string) (string, bool) {
	// TODO(andydunstall): This doesn't yet do any load balancing. It just
	// selects the first node.
	node, ok := p.networkMap.LookupEndpoint(endpointID)
	if !ok {
		return "", false
	}
	return node.ProxyAddr, true
}
