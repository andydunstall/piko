package cluster

import (
	"sync"

	"go.uber.org/zap"

	"github.com/dragonflydb/piko/pikotest/cluster/config"
	"github.com/dragonflydb/piko/pkg/log"
)

type Manager struct {
	nodes []*Node

	mu sync.Mutex

	logger log.Logger
}

func NewManager(opts ...Option) *Manager {
	options := options{
		logger: log.NewNopLogger(),
	}
	for _, o := range opts {
		o.apply(&options)
	}

	return &Manager{
		logger: options.logger.WithSubsystem("cluster.manager"),
	}
}

func (m *Manager) Update(config *config.Config) {
	m.logger.Info("update", zap.Any("config", config))

	m.mu.Lock()
	defer m.mu.Unlock()

	// Update the active nodes to ensure we have the correct number.
	if config.Nodes > len(m.nodes) {
		added := config.Nodes - len(m.nodes)
		for i := 0; i != added; i++ {
			m.addNodeLocked()
		}
	} else if len(m.nodes) > config.Nodes {
		removed := len(m.nodes) - config.Nodes
		for i := 0; i != removed; i++ {
			m.removeNodeLocked()
		}
	}
}

func (m *Manager) Nodes() []*Node {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Copy nodes to avoid race conditions when m.nodes is updated.
	var nodes []*Node
	nodes = append(nodes, m.nodes...)
	return nodes
}

func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := len(m.nodes)
	for i := 0; i != removed; i++ {
		m.removeNodeLocked()
	}
}

func (m *Manager) addNodeLocked() {
	m.logger.Info("add node")

	var gossipAddrs []string
	for _, node := range m.nodes {
		gossipAddrs = append(gossipAddrs, node.GossipAddr())
	}

	node := NewNode(WithJoin(gossipAddrs), WithLogger(m.logger))
	node.Start()

	m.nodes = append(m.nodes, node)
}

func (m *Manager) removeNodeLocked() {
	m.logger.Info("remove node")

	// Remove the oldest node.
	node := m.nodes[0]
	m.nodes = m.nodes[1:]
	node.Stop()
}
