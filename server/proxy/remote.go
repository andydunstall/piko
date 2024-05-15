package proxy

import (
	"context"
	"net/http"

	"github.com/andydunstall/piko/pkg/forwarder"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/server/cluster"
	"github.com/prometheus/client_golang/prometheus"
)

// remoteProxy is responsible for forwarding requests to Piko server nodes with
// an upstream connection for the target endpoint.
type remoteProxy struct {
	clusterState *cluster.State

	forwarder forwarder.Forwarder

	metrics *Metrics

	logger log.Logger
}

func newRemoteProxy(
	clusterState *cluster.State,
	forwarder forwarder.Forwarder,
	metrics *Metrics,
	logger log.Logger,
) *remoteProxy {
	return &remoteProxy{
		clusterState: clusterState,
		forwarder:    forwarder,
		metrics:      metrics,
		logger:       logger,
	}
}

func (p *remoteProxy) Request(
	ctx context.Context,
	endpointID string,
	r *http.Request,
) (*http.Response, error) {
	nodeID, addr, ok := p.findNode(endpointID)
	if !ok {
		return nil, errEndpointNotFound
	}
	p.metrics.ForwardedRemoteTotal.With(prometheus.Labels{
		"node_id": nodeID,
	}).Inc()
	return p.forwarder.Request(ctx, addr, r)
}

func (p *remoteProxy) AddConn(conn Conn) {
	// Update the cluster to notify other nodes that we have a connection for
	// the endpoint.
	p.clusterState.AddLocalEndpoint(conn.EndpointID())
}

func (p *remoteProxy) RemoveConn(conn Conn) {
	p.clusterState.RemoveLocalEndpoint(conn.EndpointID())
}

// findNode looks up a node with an upstream connection for the given endpoint
// and returns the node ID and proxy address.
func (p *remoteProxy) findNode(endpointID string) (string, string, bool) {
	// TODO(andydunstall): This doesn't yet do any load balancing. It just
	// selects the first node.
	node, ok := p.clusterState.LookupEndpoint(endpointID)
	if !ok {
		return "", "", false
	}
	return node.ID, node.ProxyAddr, true
}
