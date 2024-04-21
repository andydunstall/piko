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

	return m.nodes[m.localID].Copy()
}

func (m *NetworkMap) Node(id string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, false
	}
	return node.Copy(), true
}

func (m *NetworkMap) Nodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node.Copy())
	}
	return nodes
}
