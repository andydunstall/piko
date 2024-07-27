package cluster

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/andydunstall/piko/pkg/log"
)

// State represents the known state of the cluster as seen by the local
// node.
//
// This state is eventually consistent.
type State struct {
	localID string
	nodes   map[string]*Node

	localEndpointSubscribers  []func(endpointID string)
	remoteEndpointSubscribers []func(nodeID string, endpointID string)

	// mu protects the above fields.
	mu sync.RWMutex

	metrics *Metrics

	logger log.Logger
}

func NewState(
	localNode *Node,
	logger log.Logger,
) *State {
	// The local node is always active.
	localNode.Status = NodeStatusActive
	nodes := make(map[string]*Node)
	nodes[localNode.ID] = localNode

	s := &State{
		localID: localNode.ID,
		nodes:   nodes,
		metrics: NewMetrics(),
		logger:  logger.WithSubsystem("cluster"),
	}
	s.addMetricsNode(localNode.Status)
	return s
}

// Node returns the known state of the node with the given ID, or false if the
// node is unknown.
func (s *State) Node(id string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[id]
	if !ok {
		return nil, false
	}
	return node.Copy(), true
}

// LocalID returns the ID of the local node.
func (s *State) LocalID() string {
	// localID is immutable so don't need a mutex.
	return s.localID
}

// LocalNode returns the state of the local node.
func (s *State) LocalNode() *Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	node, ok := s.nodes[s.localID]
	if !ok {
		panic("local node not in cluster")
	}
	return node.Copy()
}

// Nodes returns the state of the known nodes.
func (s *State) Nodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node.Copy())
	}
	return nodes
}

// NodesMetadata returns the metadata of the known nodes.
func (s *State) NodesMetadata() []*NodeMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()

	nodes := make([]*NodeMetadata, 0, len(s.nodes))
	for _, node := range s.nodes {
		nodes = append(nodes, node.Metadata())
	}
	return nodes
}

// LookupEndpoint looks up a node that the endpoint with the given ID is active
// on.
func (s *State) LookupEndpoint(endpointID string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, node := range s.nodes {
		if node.ID == s.localID {
			// Ignore ourselves.
			continue
		}
		if node.Status != NodeStatusActive {
			// Ignore unreachable and left nodes.
			continue
		}
		if listeners, ok := node.Endpoints[endpointID]; ok && listeners > 0 {
			return node.Copy(), true
		}
	}

	return nil, false
}

// AddLocalEndpoint adds the active endpoint to the local node state.
func (s *State) AddLocalEndpoint(endpointID string) {
	s.mu.Lock()

	node, ok := s.nodes[s.localID]
	if !ok {
		panic("local node not in cluster")
	}

	if node.Endpoints == nil {
		node.Endpoints = make(map[string]int)
	}

	node.Endpoints[endpointID] = node.Endpoints[endpointID] + 1

	subscribers := make([]func(endpointID string), 0, len(s.localEndpointSubscribers))
	subscribers = append(subscribers, s.localEndpointSubscribers...)

	s.mu.Unlock()

	for _, f := range subscribers {
		f(endpointID)
	}
}

// RemoveLocalEndpoint removes the active endpoint from the local node state.
func (s *State) RemoveLocalEndpoint(endpointID string) {
	s.mu.Lock()

	node, ok := s.nodes[s.localID]
	if !ok {
		panic("local node not in cluster")
	}

	if node.Endpoints == nil {
		node.Endpoints = make(map[string]int)
	}

	listeners, ok := node.Endpoints[endpointID]
	if !ok || listeners == 0 {
		s.logger.Warn("remove local endpoint: endpoint not found")
		s.mu.Unlock()
		return
	}

	if listeners > 1 {
		node.Endpoints[endpointID] = listeners - 1
	} else {
		delete(node.Endpoints, endpointID)
	}

	subscribers := make([]func(endpointID string), 0, len(s.localEndpointSubscribers))
	subscribers = append(subscribers, s.localEndpointSubscribers...)

	s.mu.Unlock()

	for _, f := range subscribers {
		f(endpointID)
	}
}

func (s *State) LocalEndpointListeners(endpointID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.nodes[s.localID]
	if !ok {
		panic("local node not in cluster")
	}

	if node.Endpoints == nil {
		return 0
	}
	return node.Endpoints[endpointID]
}

// OnLocalEndpointUpdate subscribes to changes to the local nodes active
// endpoints.
//
// The callback is called with the cluster mutex locked so must not block or
// call back to the cluster.
func (s *State) OnLocalEndpointUpdate(f func(endpointID string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.localEndpointSubscribers = append(s.localEndpointSubscribers, f)
}

func (s *State) OnRemoteEndpointUpdate(f func(nodeID string, endpointID string)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.remoteEndpointSubscribers = append(s.remoteEndpointSubscribers, f)
}

// AddNode adds the given node to the cluster.
func (s *State) AddNode(node *Node) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if node.ID == s.localID {
		s.logger.Warn("add node: cannot add local node")
		return
	}

	if _, ok := s.nodes[node.ID]; ok {
		// If already in the cluster update the node but warn as this should
		// not happen.
		s.logger.Warn("add node: node already in cluster")
	}

	s.nodes[node.ID] = node
	s.addMetricsNode(node.Status)
}

// RemoveNode removes the node with the given ID from the cluster.
func (s *State) RemoveNode(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == s.localID {
		s.logger.Warn("remove node: cannot remove local node")
		return false
	}

	node, ok := s.nodes[id]
	if !ok {
		s.logger.Warn("remove node: node not in cluster")
		return false
	}

	delete(s.nodes, id)
	s.removeMetricsNode(node.Status)

	return true
}

// UpdateRemoteStatus sets the status of the remote node with the given ID.
func (s *State) UpdateRemoteStatus(id string, status NodeStatus) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id == s.localID {
		s.logger.Warn("update remote status: cannot update local node")
		return false
	}

	n, ok := s.nodes[id]
	if !ok {
		s.logger.Warn("update remote status: node not in cluster")
		return false
	}

	oldStatus := n.Status
	n.Status = status
	s.updateMetricsNode(oldStatus, status)
	return true
}

// UpdateRemoteEndpoint sets the number of listeners for the active endpoint
// for the node with the given ID.
func (s *State) UpdateRemoteEndpoint(
	id string,
	endpointID string,
	listeners int,
) bool {
	s.mu.Lock()

	if !s.updateRemoteEndpointLocked(id, endpointID, listeners) {
		s.mu.Unlock()
		return false
	}

	subscribers := make([]func(nodeID string, endpointID string), 0, len(s.remoteEndpointSubscribers))
	subscribers = append(subscribers, s.remoteEndpointSubscribers...)

	s.mu.Unlock()

	for _, f := range subscribers {
		f(id, endpointID)
	}

	return true
}

// RemoveRemoteEndpoint removes the active endpoint from the node with the
// given ID.
func (s *State) RemoveRemoteEndpoint(id string, endpointID string) bool {
	s.mu.Lock()

	if !s.removeRemoteEndpointLocked(id, endpointID) {
		s.mu.Unlock()
		return false
	}

	subscribers := make([]func(nodeID string, endpointID string), 0, len(s.remoteEndpointSubscribers))
	subscribers = append(subscribers, s.remoteEndpointSubscribers...)

	s.mu.Unlock()

	for _, f := range subscribers {
		f(id, endpointID)
	}

	return true
}

func (s *State) Metrics() *Metrics {
	return s.metrics
}

func (s *State) updateRemoteEndpointLocked(
	id string,
	endpointID string,
	listeners int,
) bool {
	if id == s.localID {
		s.logger.Warn("update remote endpoint: cannot update local node")
		return false
	}

	n, ok := s.nodes[id]
	if !ok {
		s.logger.Warn("update remote endpoint: node not in cluster")
		return false
	}

	if n.Endpoints == nil {
		n.Endpoints = make(map[string]int)
	}

	n.Endpoints[endpointID] = listeners

	return true
}

func (s *State) removeRemoteEndpointLocked(id string, endpointID string) bool {
	if id == s.localID {
		s.logger.Warn("remove remote endpoint: cannot update local node")
		return false
	}

	n, ok := s.nodes[id]
	if !ok {
		s.logger.Warn("remove remote endpoint: node not in cluster")
		return false
	}

	if n.Endpoints != nil {
		delete(n.Endpoints, endpointID)
	}

	return true
}

func (s *State) updateMetricsNode(oldStatus NodeStatus, newStatus NodeStatus) {
	s.removeMetricsNode(oldStatus)
	s.addMetricsNode(newStatus)
}

func (s *State) addMetricsNode(status NodeStatus) {
	s.metrics.Nodes.With(prometheus.Labels{"status": string(status)}).Inc()
}

func (s *State) removeMetricsNode(status NodeStatus) {
	s.metrics.Nodes.With(prometheus.Labels{"status": string(status)}).Dec()
}

func (s *State) TotalAndLocalUpstreams() (int, int) {
	totalUpstreams := 0
	localUpstreams := 0
	for _, n := range s.NodesMetadata() {
		if n.ID == s.localID {
			localUpstreams = n.Upstreams
		}
		totalUpstreams += n.Upstreams
	}
	return totalUpstreams, localUpstreams
}

func (s *State) NodesNum() int {
	return len(s.nodes)
}
