package netmap

import (
	"sync"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/prometheus/client_golang/prometheus"
)

// NetworkMap represents the known state of the cluster as seen by the local
// node.
//
// This map is eventually consistent. The state is propagated among the nodes
// in the cluster using gossip.
type NetworkMap struct {
	localID string
	nodes   map[string]*Node

	localEndpointSubscribers []func(endpointID string, listeners int)

	// mu protects the above fields.
	mu sync.RWMutex

	metrics *Metrics

	logger log.Logger
}

func NewNetworkMap(
	localNode *Node,
	logger log.Logger,
) *NetworkMap {
	// The local node is always active.
	localNode.Status = NodeStatusActive
	nodes := make(map[string]*Node)
	nodes[localNode.ID] = localNode

	m := &NetworkMap{
		localID: localNode.ID,
		nodes:   nodes,
		metrics: NewMetrics(),
		logger:  logger.WithSubsystem("netmap"),
	}
	m.addMetricsEntry(localNode.Status)
	return m
}

// Node returns the known state of the node with the given ID, or false if the
// node is unknown.
func (m *NetworkMap) Node(id string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, false
	}
	return node.Copy(), true
}

// LocalID returns the ID of the local node.
func (m *NetworkMap) LocalID() string {
	// localID is immutable so don't need a mutex.
	return m.localID
}

// LocalNode returns the state of the local node.
func (m *NetworkMap) LocalNode() *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[m.localID]
	if !ok {
		panic("local node not in netmap")
	}
	return node.Copy()
}

// Nodes returns the state of the known nodes.
func (m *NetworkMap) Nodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node.Copy())
	}
	return nodes
}

func (m *NetworkMap) LookupEndpoint(endpointID string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range m.nodes {
		if node.ID == m.localID {
			// Ignore ourselves.
			continue
		}
		if listeners, ok := node.Endpoints[endpointID]; ok && listeners > 0 {
			return node.Copy(), true
		}
	}

	return nil, false
}

// AddLocalEndpoint adds the active endpoint to the local node state.
func (m *NetworkMap) AddLocalEndpoint(endpointID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[m.localID]
	if !ok {
		panic("local node not in netmap")
	}

	if node.Endpoints == nil {
		node.Endpoints = make(map[string]int)
	}

	node.Endpoints[endpointID] = node.Endpoints[endpointID] + 1

	for _, f := range m.localEndpointSubscribers {
		f(endpointID, node.Endpoints[endpointID])
	}
}

// RemoveLocalEndpoint removes the active endpoint from the local node state.
func (m *NetworkMap) RemoveLocalEndpoint(endpointID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[m.localID]
	if !ok {
		panic("local node not in netmap")
	}

	if node.Endpoints == nil {
		node.Endpoints = make(map[string]int)
	}

	listeners, ok := node.Endpoints[endpointID]
	if !ok || listeners == 0 {
		m.logger.Warn("remove local endpoint: endpoint not found")
		return
	}

	if listeners > 1 {
		node.Endpoints[endpointID] = listeners - 1
	} else {
		delete(node.Endpoints, endpointID)
	}

	for _, f := range m.localEndpointSubscribers {
		f(endpointID, node.Endpoints[endpointID])
	}
}

// OnLocalEndpointUpdate subscribes to changes to the local nodes active
// endpoints.
//
// The callback is called with the netmap mutex locked so must not block or
// call back to the netmap.
func (m *NetworkMap) OnLocalEndpointUpdate(f func(endpointID string, listeners int)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.localEndpointSubscribers = append(m.localEndpointSubscribers, f)
}

// AddNode adds the given node to the netmap.
func (m *NetworkMap) AddNode(node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node.ID == m.localID {
		m.logger.Warn("add node: cannot add local node")
		return
	}

	if _, ok := m.nodes[node.ID]; ok {
		// If already in the netmap update the node but warn as this should
		// not happen.
		m.logger.Warn("add node: node already in netmap")
	}

	m.nodes[node.ID] = node
	m.addMetricsEntry(node.Status)
}

// RemoveNode removes the node with the given ID from the netmap.
func (m *NetworkMap) RemoveNode(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == m.localID {
		m.logger.Warn("remove node: cannot remove local node")
		return false
	}

	node, ok := m.nodes[id]
	if !ok {
		m.logger.Warn("remove node: node not in netmap")
		return false
	}

	delete(m.nodes, id)
	m.removeMetricsEntry(node.Status)

	return true
}

// UpdateRemoteStatus sets the status of the remote node with the given ID.
func (m *NetworkMap) UpdateRemoteStatus(id string, status NodeStatus) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == m.localID {
		m.logger.Warn("update remote status: cannot update local node")
		return false
	}

	n, ok := m.nodes[id]
	if !ok {
		m.logger.Warn("update remote status: node not in netmap")
		return false
	}

	oldStatus := n.Status
	n.Status = status
	m.updateMetricsEntry(oldStatus, status)
	return true
}

// UpdateRemoteEndpoint sets the number of listeners for the active endpoint
// for the node with the given ID.
func (m *NetworkMap) UpdateRemoteEndpoint(
	id string,
	endpointID string,
	listeners int,
) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == m.localID {
		m.logger.Warn("update remote endpoint: cannot update local node")
		return false
	}

	n, ok := m.nodes[id]
	if !ok {
		m.logger.Warn("update remote endpoint: node not in netmap")
		return false
	}

	if n.Endpoints == nil {
		n.Endpoints = make(map[string]int)
	}

	n.Endpoints[endpointID] = listeners

	return true
}

// RemoveRemoteEndpoint removes the active endpoint from the node with the
// given ID.
func (m *NetworkMap) RemoveRemoteEndpoint(id string, endpointID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == m.localID {
		m.logger.Warn("remove remote endpoint: cannot update local node")
		return false
	}

	n, ok := m.nodes[id]
	if !ok {
		m.logger.Warn("remove remote endpoint: node not in netmap")
		return false
	}

	if n.Endpoints != nil {
		delete(n.Endpoints, endpointID)
	}

	return true
}

func (m *NetworkMap) Metrics() *Metrics {
	return m.metrics
}

func (m *NetworkMap) updateMetricsEntry(oldStatus NodeStatus, newStatus NodeStatus) {
	m.removeMetricsEntry(oldStatus)
	m.addMetricsEntry(newStatus)
}

func (m *NetworkMap) addMetricsEntry(s NodeStatus) {
	m.metrics.Entries.With(prometheus.Labels{"status": string(s)}).Inc()
}

func (m *NetworkMap) removeMetricsEntry(s NodeStatus) {
	m.metrics.Entries.With(prometheus.Labels{"status": string(s)}).Dec()
}
