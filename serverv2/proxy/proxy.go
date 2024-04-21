package proxy

import "github.com/andydunstall/pico/pkg/rpc"

// Proxy is responsible for forwarding requests to upstream listeners.
type Proxy struct {
}

func NewProxy() *Proxy {
	return &Proxy{}
}

func (p *Proxy) AddUpstream(_ string, _ *rpc.Stream) {
}

func (p *Proxy) RemoveUpstream(_ string, _ *rpc.Stream) {
}
