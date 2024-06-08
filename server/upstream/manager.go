package upstream

import (
	"sync"

	"github.com/andydunstall/piko/server/cluster"
)

// Manager manages the upstream routes for each endpoint.
//
// This includes upstreams connected to the local node, or other server nodes
// in the cluster with a connected upstream for the target endpoint.
type Manager interface {
	// Select looks up an upstream for the given endpoint ID.
	//
	// This will first look for an upstream connected to the local node, and
	// load balance among the available connected upstreams.
	//
	// If there are no upstreams connected for the endpoint, and 'allowForward'
	// is true, it will look for another node in the cluster that has an
	// upstream connection for the endpoint and use that node as the upstream.
	Select(endpointID string, allowForward bool) (Upstream, bool)

	// AddConn adds a local upstream connection.
	AddConn(u Upstream)

	// RemoveConn removes a local upstream connection.
	RemoveConn(u Upstream)
}

// loadBalancer load balances requests among upstreams in a round-robin
// fashion.
type loadBalancer struct {
	upstreams []Upstream
	nextIndex int
}

func (lb *loadBalancer) Add(u Upstream) {
	lb.upstreams = append(lb.upstreams, u)
}

func (lb *loadBalancer) Remove(u Upstream) bool {
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

func (lb *loadBalancer) Next() Upstream {
	if len(lb.upstreams) == 0 {
		return nil
	}

	u := lb.upstreams[lb.nextIndex]
	lb.nextIndex++
	lb.nextIndex %= len(lb.upstreams)
	return u
}

type manager struct {
	localUpstreams map[string]*loadBalancer

	cluster *cluster.State

	mu sync.Mutex
}

func NewManager(cluster *cluster.State) Manager {
	return &manager{
		localUpstreams: make(map[string]*loadBalancer),
		cluster:        cluster,
	}
}

func (m *manager) Select(endpointID string, allowRemote bool) (Upstream, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[endpointID]
	if ok {
		return lb.Next(), true
	}
	if !allowRemote {
		return nil, false
	}

	node, ok := m.cluster.LookupEndpoint(endpointID)
	if !ok {
		return nil, false
	}
	return NewNodeUpstream(endpointID, node), true
}

func (m *manager) AddConn(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[u.EndpointID()]
	if !ok {
		lb = &loadBalancer{}
	}

	lb.Add(u)
	m.localUpstreams[u.EndpointID()] = lb

	m.cluster.AddLocalEndpoint(u.EndpointID())
}

func (m *manager) RemoveConn(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[u.EndpointID()]
	if !ok {
		return
	}
	if lb.Remove(u) {
		delete(m.localUpstreams, u.EndpointID())
	}

	m.cluster.RemoveLocalEndpoint(u.EndpointID())
}
