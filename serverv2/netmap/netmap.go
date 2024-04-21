package netmap

import (
	"sync"

	"github.com/andydunstall/pico/pkg/log"
	"go.uber.org/zap"
)

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

	logger *log.Logger
}

func NewNetworkMap(localNode *Node, logger *log.Logger) *NetworkMap {
	nodes := make(map[string]*Node)
	nodes[localNode.ID] = localNode
	return &NetworkMap{
		localID: localNode.ID,
		nodes:   nodes,
		logger:  logger.WithSubsystem("netmap"),
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

func (m *NetworkMap) AddRemote(node *Node) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes[node.ID] = node

	m.logger.Debug(
		"add remote ",
		zap.String("node-id", node.ID),
		zap.String("status", string(node.Status)),
	)
}

func (m *NetworkMap) RemoveRemote(nodeID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[nodeID]; ok {
		delete(m.nodes, nodeID)
		m.logger.Debug(
			"remove remote ",
			zap.String("node-id", nodeID),
		)
		return true
	}
	return false
}

func (m *NetworkMap) UpdateRemote(nodeID, key, value string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return false
	}

	m.nodes[node.ID] = node

	switch key {
	case "status":
		node.Status = NodeStatus(value)

		m.logger.Debug(
			"update remote; status updated ",
			zap.String("node-id", nodeID),
			zap.String("status", string(node.Status)),
		)
	default:
		m.logger.Warn(
			"update remote; unknown key ",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
	}

	return true
}
