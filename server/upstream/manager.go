package upstream

import (
	"crypto/tls"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

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

type Usage struct {
	Requests  *atomic.Uint64
	Upstreams *atomic.Uint64
}

type LoadBalancedManager struct {
	localUpstreams map[string]*loadBalancer

	mu sync.Mutex

	usage *Usage

	cluster *cluster.State

	metrics *Metrics

	tlsConfig *tls.Config
}

func NewLoadBalancedManager(cluster *cluster.State, proxyClientTLSConfig *tls.Config) *LoadBalancedManager {
	return &LoadBalancedManager{
		localUpstreams: make(map[string]*loadBalancer),
		cluster:        cluster,
		tlsConfig:      proxyClientTLSConfig,
		usage: &Usage{
			Requests:  atomic.NewUint64(0),
			Upstreams: atomic.NewUint64(0),
		},
		metrics: NewMetrics(),
	}
}

func (m *LoadBalancedManager) Select(endpointID string, allowRemote bool) (Upstream, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[endpointID]
	if ok {
		m.metrics.UpstreamRequestsTotal.Inc()
		return lb.Next(), true
	}
	if !allowRemote {
		return nil, false
	}

	node, ok := m.cluster.LookupEndpoint(endpointID)
	if !ok {
		return nil, false
	}
	m.metrics.RemoteRequestsTotal.With(prometheus.Labels{
		"node_id": node.ID,
	}).Inc()
	m.usage.Requests.Inc()
	return NewNodeUpstream(endpointID, node, m.tlsConfig), true
}

func (m *LoadBalancedManager) AddConn(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[u.EndpointID()]
	if !ok {
		lb = &loadBalancer{}

		m.metrics.RegisteredEndpoints.Inc()
	}

	lb.Add(u)
	m.localUpstreams[u.EndpointID()] = lb

	m.cluster.AddLocalEndpoint(u.EndpointID())

	m.metrics.ConnectedUpstreams.Inc()
	m.usage.Upstreams.Inc()
}

func (m *LoadBalancedManager) RemoveConn(u Upstream) {
	m.mu.Lock()
	defer m.mu.Unlock()

	lb, ok := m.localUpstreams[u.EndpointID()]
	if !ok {
		return
	}
	if lb.Remove(u) {
		delete(m.localUpstreams, u.EndpointID())

		m.metrics.RegisteredEndpoints.Dec()
	}

	m.cluster.RemoveLocalEndpoint(u.EndpointID())

	m.metrics.ConnectedUpstreams.Dec()
}

func (m *LoadBalancedManager) Endpoints() map[string]int {
	m.mu.Lock()
	defer m.mu.Unlock()

	endpoints := make(map[string]int)
	for endpointID, lb := range m.localUpstreams {
		endpoints[endpointID] = len(lb.upstreams)
	}
	return endpoints
}

func (m *LoadBalancedManager) Usage() *Usage {
	return m.usage
}

func (m *LoadBalancedManager) Metrics() *Metrics {
	return m.metrics
}
