package proxy

import (
	"github.com/andydunstall/pico/pkg/rpc"
	"github.com/andydunstall/pico/serverv2/netmap"
)

// Proxy is responsible for forwarding requests to upstream listeners.
type Proxy struct {
	networkMap *netmap.NetworkMap
}

func NewProxy(networkMap *netmap.NetworkMap) *Proxy {
	return &Proxy{
		networkMap: networkMap,
	}
}

func (p *Proxy) AddUpstream(endpointID string, _ *rpc.Stream) {
	p.networkMap.AddLocalEndpoint(endpointID)
}

func (p *Proxy) RemoveUpstream(endpointID string, _ *rpc.Stream) {
	p.networkMap.RemoveLocalEndpoint(endpointID)
}
