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

	// mu protects the above fields.
	mu sync.RWMutex
}

func NewNetworkMap(localNode *Node) *NetworkMap {
	nodes := make(map[string]*Node)
	nodes[localNode.ID] = localNode
	return &NetworkMap{
		localID: localNode.ID,
		nodes:   nodes,
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
	f(node)
	return true
}

func (m *NetworkMap) DeleteNodeByID(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.nodes[id]
	delete(m.nodes, id)
	return ok
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
