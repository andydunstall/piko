package cluster

import (
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/workloadv2/cluster/config"
	"go.uber.org/zap"
)

type Manager struct {
	nodes []*Node

	logger log.Logger
}

func NewManager(logger log.Logger) *Manager {
	return &Manager{
		logger: logger.WithSubsystem("cluster.manager"),
	}
}

func (m *Manager) Update(config *config.Config) {
	m.logger.Info("update", zap.Any("config", config))

	// Update the active nodes to ensure we have the correct number.
	if config.Nodes > len(m.nodes) {
		added := config.Nodes - len(m.nodes)
		for i := 0; i != added; i++ {
			m.addNode()
		}
	} else if len(m.nodes) > config.Nodes {
		removed := len(m.nodes) - config.Nodes
		for i := 0; i != removed; i++ {
			m.removeNode()
		}
	}
}

func (m *Manager) Nodes() []*Node {
	return m.nodes
}

func (m *Manager) Close() {
	removed := len(m.nodes)
	for i := 0; i != removed; i++ {
		m.removeNode()
	}
}

func (m *Manager) addNode() {
	m.logger.Info("add node")

	var gossipAddrs []string
	for _, node := range m.nodes {
		gossipAddrs = append(gossipAddrs, node.GossipAddr())
	}

	node := NewNode(gossipAddrs, m.logger)
	node.Start()

	m.nodes = append(m.nodes, node)
}

func (m *Manager) removeNode() {
	m.logger.Info("remove node")

	// Remove the oldest node.
	node := m.nodes[0]
	m.nodes = m.nodes[1:]
	node.Stop()
}
