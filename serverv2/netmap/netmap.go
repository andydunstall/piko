package netmap

import (
	"strconv"
	"strings"
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

	localUpsertSubscribers []func(key, value string)

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

func (m *NetworkMap) UpdateLocalStatus(status NodeStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes[m.localID].Status = status

	for _, f := range m.localUpsertSubscribers {
		f("status", string(status))
	}

	m.logger.Debug(
		"updated local status",
		zap.String("status", string(status)),
	)
}

func (m *NetworkMap) AddLocalEndpoint(endpointID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	numListeners := m.nodes[m.localID].AddEndpoint(endpointID)

	key := "endpoint:" + endpointID
	for _, f := range m.localUpsertSubscribers {
		f(key, strconv.Itoa(numListeners))
	}

	m.logger.Debug(
		"added local endpoint",
		zap.String("endpoint-id", endpointID),
		zap.Int("num-listeners", numListeners),
	)
}

func (m *NetworkMap) RemoveLocalEndpoint(endpointID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	numListeners := m.nodes[m.localID].RemoveEndpoint(endpointID)

	key := "endpoint:" + endpointID
	for _, f := range m.localUpsertSubscribers {
		if numListeners > 0 {
			f(key, strconv.Itoa(numListeners))
		} else {
			f(key, "")
		}
	}

	m.logger.Debug(
		"removed local endpoint",
		zap.String("endpoint-id", endpointID),
		zap.Int("num-listeners", numListeners),
	)
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

func (m *NetworkMap) UpsertRemote(nodeID, key, value string) bool {
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
		if strings.HasPrefix(key, "endpoint:") {
			endpointID, _ := strings.CutPrefix(key, "endpoint:")
			numListeners, err := strconv.Atoi(value)
			if err != nil {
				m.logger.Error(
					"invalid endpoint: num listeners",
					zap.String("node-id", node.ID),
					zap.Error(err),
				)
			}
			if node.Endpoints == nil {
				node.Endpoints = make(map[string]int)
			}
			node.Endpoints[endpointID] = numListeners

			m.logger.Debug(
				"update remote; endpoint updated ",
				zap.String("node-id", nodeID),
				zap.String("endpoint-id", endpointID),
			)
		} else {
			m.logger.Error(
				"unknown key",
				zap.String("node-id", node.ID),
				zap.String("key", key),
			)
		}
	}

	return true
}

func (m *NetworkMap) DeleteRemoteState(nodeID, key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[nodeID]
	if !ok {
		return false
	}

	if strings.HasPrefix(key, "endpoint:") {
		endpointID, _ := strings.CutPrefix(key, "endpoint:")
		if node.Endpoints != nil {
			delete(node.Endpoints, endpointID)
		}
	} else {
		m.logger.Error(
			"unknown key",
			zap.String("node-id", node.ID),
			zap.String("key", key),
		)
	}

	return true
}

// OnLocalUpsert subscribes to updates to the local node. The callback must not
// block.
func (m *NetworkMap) OnLocalUpsert(f func(key, value string)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.localUpsertSubscribers = append(m.localUpsertSubscribers, f)
}

func (m *NetworkMap) LookupEndpoint(endpointID string) (*Node, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, node := range m.nodes {
		if numListeners, ok := node.Endpoints[endpointID]; ok && numListeners > 0 {
			return node.Copy(), true
		}
	}

	return nil, false
}
