package netmap

import "sync"

// NetworkMap represents the known state of the cluster as seen by the local
// node.
//
// This map is eventually consistent. The state is propagated among the nodes
// in the cluster using gossip.
type NetworkMap struct {
	localID string
	nodes   map[string]*Node

	// endpointIndex maps each active endpoint ID to a set of node IDs that
	// have at least one listener for that endpoint.
	endpointIndex map[string]map[string]struct{}

	// mu protects the above fields.
	mu sync.RWMutex
}

func NewNetworkMap(localNode *Node) *NetworkMap {
	nodes := make(map[string]*Node)
	nodes[localNode.ID] = localNode
	return &NetworkMap{
		localID:       localNode.ID,
		nodes:         nodes,
		endpointIndex: make(map[string]map[string]struct{}),
	}
}

func (m *NetworkMap) LocalNode() *Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.nodes[m.localID]
}

func (m *NetworkMap) NodeByID(id string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	return node, ok
}

func (m *NetworkMap) AddNode(node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes[node.ID] = node

	m.addToEndpointIndex(node)
}

// UpdateNodeByID updates the node with the given ID using a callback function.
//
// The function must not block as it is called while the netmap mutex is held.
func (m *NetworkMap) UpdateNodeByID(id string, f func(n *Node)) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return false
	}
	// Remove this nodes endpoints before updating the node as a simple way to
	// keep the index up to date.
	// TODO(andydunstall): This will be expensive with 1000s of endpoints per
	// node so look at improving. Can add an explicit
	// netmap.AddEndpoint(nodeID, endpointID).
	m.removeFromEndpointIndex(node)
	f(node)
	m.addToEndpointIndex(node)
	return true
}

func (m *NetworkMap) DeleteNodeByID(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	m.removeFromEndpointIndex(node)
	delete(m.nodes, id)
	return ok
}

func (m *NetworkMap) NodesByEndpointID(endpointID string) []*Node {
	m.mu.Lock()
	defer m.mu.Unlock()

	nodeIDs, ok := m.endpointIndex[endpointID]
	if !ok {
		return nil
	}

	var nodes []*Node
	for nodeID := range nodeIDs {
		nodes = append(nodes, m.nodes[nodeID])
	}
	return nodes
}

func (m *NetworkMap) OnLocalStatusUpdated(_ func(status NodeStatus)) {
	// TODO(andydunstall)
}

func (m *NetworkMap) OnLocalEndpointUpdated(_ func(endpointID string, numListeners int)) {
	// TODO(andydunstall)
}

func (m *NetworkMap) OnLocalEndpointRemoved(_ func(endpointID string)) {
	// TODO(andydunstall)
}

// addToEndpointIndex adds the endpoints for the given node to the endpoint
// index. Note the mutex MUST be held before calling this method.
func (m *NetworkMap) addToEndpointIndex(node *Node) {
	for endpointID, numListeners := range node.Endpoints {
		if numListeners <= 0 {
			continue
		}
		nodeIDs, ok := m.endpointIndex[endpointID]
		if !ok {
			nodeIDs = make(map[string]struct{})
		}
		nodeIDs[node.ID] = struct{}{}
		m.endpointIndex[endpointID] = nodeIDs
	}
}

// removeFromEndpointIndex removes the endpoints for the given node to the
// endpoint index. Note the mutex MUST be held before calling this method.
func (m *NetworkMap) removeFromEndpointIndex(node *Node) {
	for endpointID := range node.Endpoints {
		nodeIDs, ok := m.endpointIndex[endpointID]
		if ok {
			delete(nodeIDs, node.ID)
		}
	}
}
