package upstream

import (
	"sync"
)

// localLoadBalancer load balances requests among upstreams connected to
// the local node.
type localLoadBalancer struct {
	upstreams []Upstream
	nextIndex int
}

func (lb *localLoadBalancer) Add(u Upstream) {
	lb.upstreams = append(lb.upstreams, u)
}

func (lb *localLoadBalancer) Remove(u Upstream) bool {
	for i := 0; i != len(lb.upstreams); i++ {
		if lb.upstreams[i] != u {
			continue
		}
		lb.upstreams = append(lb.upstreams[:i], lb.upstreams[i+1:]...)
		if len(lb.upstreams) == 0 {
			return true
		}
		lb.nextIndex %= len(lb.upstreams)
		return false
	}
	return len(lb.upstreams) == 0
}

func (lb *localLoadBalancer) Next() Upstream {
	if len(lb.upstreams) == 0 {
		return nil
	}

	u := lb.upstreams[lb.nextIndex]
	lb.nextIndex++
	lb.nextIndex %= len(lb.upstreams)
	return u
}

// Manager manages the set of local upsteam services.
type Manager struct {
	localLoadBalancers map[string]*localLoadBalancer

	mu sync.Mutex
}

func NewManager() *Manager {
	return &Manager{
		localLoadBalancers: make(map[string]*localLoadBalancer),
	}
}

func (m *Manager) Add(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localLoadBalancers[u.EndpointID()]
	if !ok {
		lb = &localLoadBalancer{}
	}

	lb.Add(u)
	m.localLoadBalancers[u.EndpointID()] = lb
}

func (m *Manager) Remove(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localLoadBalancers[u.EndpointID()]
	if !ok {
		return
	}
	if lb.Remove(u) {
		delete(m.localLoadBalancers, u.EndpointID())
	}
}

func (m *Manager) Select(endpointID string, _ bool) (Upstream, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO(andydunstall): If allowForward can select from another node, where
	// Dial is just TCP

	lb, ok := m.localLoadBalancers[endpointID]
	if !ok {
		return nil, false
	}
	return lb.Next(), true
}
