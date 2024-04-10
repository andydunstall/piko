package proxy

import (
	"math/rand"

	"github.com/andydunstall/pico/server/netmap"
)

type EndpointResolver struct {
	networkMap *netmap.NetworkMap
}

func NewEndpointResolver(networkMap *netmap.NetworkMap) *EndpointResolver {
	return &EndpointResolver{
		networkMap: networkMap,
	}
}

func (r *EndpointResolver) Resolve(endpointID string) (string, bool) {
	nodes := r.networkMap.NodesByEndpointID(endpointID)
	if len(nodes) == 0 {
		return "", false
	}
	// TODO(andydunstall): Use the number of listeners each node has as a
	// weight.
	node := nodes[rand.Int()%len(nodes)]
	return node.HTTPAddr, true
}

func (r *EndpointResolver) AddLocalUpstream(endpointID string) {
	r.networkMap.UpdateNodeByID(r.networkMap.LocalNode().ID, func(n *netmap.Node) {
		if n.Endpoints == nil {
			n.Endpoints = make(map[string]int)
		}
		n.Endpoints[endpointID] = n.Endpoints[endpointID] + 1
	})
}

func (r *EndpointResolver) RemoveLocalUpstream(endpointID string) {
	r.networkMap.UpdateNodeByID(r.networkMap.LocalNode().ID, func(n *netmap.Node) {
		numListeners, ok := n.Endpoints[endpointID]
		if ok {
			n.Endpoints[endpointID] = numListeners - 1
			if n.Endpoints[endpointID] <= 0 {
				delete(n.Endpoints, endpointID)
			}
		}
	})
}
