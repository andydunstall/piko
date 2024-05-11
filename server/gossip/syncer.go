package gossip

import (
	"strconv"
	"strings"
	"sync"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/cluster"
	"go.uber.org/zap"
)

type gossiper interface {
	UpsertLocal(key, value string)
	DeleteLocal(key string)
}

// syncer handles syncronising state between gossip and the cluster.
//
// When a node joins, it is considered 'pending' so not added to the cluster
// until we have the full node state. Since gossip propagates state updates in
// order, we only add a node to the cluster when we have the required immutable
// fields.
type syncer struct {
	// pendingNodes contains nodes that we haven't received the full state for
	// yet so can't be added to the cluster.
	pendingNodes map[string]*cluster.Node

	// mu protects the above fields.
	mu sync.Mutex

	clusterState *cluster.State

	gossiper gossiper

	logger log.Logger
}

func newSyncer(clusterState *cluster.State, logger log.Logger) *syncer {
	return &syncer{
		pendingNodes: make(map[string]*cluster.Node),
		clusterState: clusterState,
		logger:       logger,
	}
}

func (s *syncer) Sync(gossiper gossiper) {
	s.gossiper = gossiper

	s.clusterState.OnLocalEndpointUpdate(s.onLocalEndpointUpdate)

	localNode := s.clusterState.LocalNode()
	// First add immutable fields.
	s.gossiper.UpsertLocal("proxy_addr", localNode.ProxyAddr)
	s.gossiper.UpsertLocal("admin_addr", localNode.AdminAddr)
	// Finally add mutable fields.
	for endpointID, listeners := range localNode.Endpoints {
		key := "endpoint:" + endpointID
		s.gossiper.UpsertLocal(key, strconv.Itoa(listeners))
	}
}

func (s *syncer) OnJoin(nodeID string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node joined; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if _, ok := s.clusterState.Node(nodeID); ok {
		s.logger.Warn(
			"node joined; already in cluster",
			zap.String("node-id", nodeID),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.pendingNodes[nodeID]; ok {
		s.logger.Warn(
			"node joined; already pending",
			zap.String("node-id", nodeID),
		)
		return
	}

	// Add as pending since we don't have enough information to add to the
	// cluster.
	s.pendingNodes[nodeID] = &cluster.Node{
		ID: nodeID,
	}

	s.logger.Info("node joined", zap.String("node-id", nodeID))
}

func (s *syncer) OnLeave(nodeID string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node healthy; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if updated := s.clusterState.UpdateRemoteStatus(nodeID, cluster.NodeStatusLeft); updated {
		s.logger.Info(
			"node leave; updated cluster",
			zap.String("node-id", nodeID),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// If a pending node has left it can be discarded.
	_, ok := s.pendingNodes[nodeID]
	if ok {
		delete(s.pendingNodes, nodeID)

		s.logger.Info(
			"node leave; removed from pending",
			zap.String("node-id", nodeID),
		)
	} else {
		s.logger.Warn(
			"node left; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (s *syncer) OnUp(nodeID string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node up; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if updated := s.clusterState.UpdateRemoteStatus(nodeID, cluster.NodeStatusActive); updated {
		s.logger.Info(
			"node up; updated cluster",
			zap.String("node-id", nodeID),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pending, ok := s.pendingNodes[nodeID]
	if ok {
		pending.Status = cluster.NodeStatusActive

		s.logger.Info(
			"node up; updated pending",
			zap.String("node-id", nodeID),
		)
	} else {
		s.logger.Warn(
			"node up; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (s *syncer) OnDown(nodeID string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node down; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if updated := s.clusterState.UpdateRemoteStatus(
		nodeID, cluster.NodeStatusUnreachable,
	); updated {
		s.logger.Info(
			"node down; updated cluster",
			zap.String("node-id", nodeID),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update pending status. We must still retain the pending node as it may
	// come back.
	pending, ok := s.pendingNodes[nodeID]
	if ok {
		pending.Status = cluster.NodeStatusUnreachable

		s.logger.Info(
			"node down; updated pending",
			zap.String("node-id", nodeID),
		)
	} else {
		s.logger.Warn(
			"node down; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (s *syncer) OnExpired(nodeID string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node expired; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if removed := s.clusterState.RemoveNode(nodeID); removed {
		s.logger.Info(
			"node expired; removed from cluster",
			zap.String("node-id", nodeID),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, ok := s.pendingNodes[nodeID]
	if ok {
		delete(s.pendingNodes, nodeID)

		s.logger.Info(
			"node expired; removed from pending",
			zap.String("node-id", nodeID),
		)
	} else {
		s.logger.Warn(
			"node expired; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (s *syncer) OnUpsertKey(nodeID, key, value string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node upsert state; same id as local node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	// First check if the node is already in the cluster. Only check mutable
	// fields.
	if strings.HasPrefix(key, "endpoint:") {
		endpointID, _ := strings.CutPrefix(key, "endpoint:")
		listeners, err := strconv.Atoi(value)
		if err != nil {
			s.logger.Error(
				"node upsert state; invalid endpoint listeners",
				zap.String("node-id", nodeID),
				zap.String("listeners", value),
				zap.Error(err),
			)
			return
		}
		if s.clusterState.UpdateRemoteEndpoint(nodeID, endpointID, listeners) {
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.pendingNodes[nodeID]
	if !ok {
		s.logger.Warn(
			"node upsert state; unknown node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
		return
	} else if key == "proxy_addr" {
		node.ProxyAddr = value
	} else if key == "admin_addr" {
		node.AdminAddr = value
	} else if strings.HasPrefix(key, "endpoint:") {
		endpointID, _ := strings.CutPrefix(key, "endpoint:")
		listeners, err := strconv.Atoi(value)
		if err != nil {
			s.logger.Error(
				"node upsert state; invalid endpoint listeners",
				zap.String("node-id", nodeID),
				zap.String("listeners", value),
				zap.Error(err),
			)
			return
		}
		if node.Endpoints == nil {
			node.Endpoints = make(map[string]int)
		}
		node.Endpoints[endpointID] = listeners
	} else {
		s.logger.Error(
			"node upsert state; unsupported key",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	// Once we have the nodes immutable fields it can be added to the cluster.
	if node.ProxyAddr != "" && node.AdminAddr != "" {
		if node.Status == "" {
			// Unless we've received a down/leave notification, we consider
			// the node as active.
			node.Status = cluster.NodeStatusActive
		}

		delete(s.pendingNodes, node.ID)
		s.clusterState.AddNode(node)

		s.logger.Debug(
			"node upsert state; added to cluster",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
	} else {
		s.logger.Debug(
			"node upsert state; updated pending node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
	}
}

func (s *syncer) OnDeleteKey(nodeID, key string) {
	if nodeID == s.clusterState.LocalID() {
		s.logger.Warn(
			"node delete state; same id as local node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	// Only endpoint state can be deleted.
	if !strings.HasPrefix(key, "endpoint:") {
		s.logger.Error(
			"node delete state; unsupported key",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	endpointID, _ := strings.CutPrefix(key, "endpoint:")
	if s.clusterState.RemoveRemoteEndpoint(nodeID, endpointID) {
		s.logger.Debug(
			"node delete state; cluster updated",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	node, ok := s.pendingNodes[nodeID]
	if !ok {
		s.logger.Warn(
			"node delete state; unknown node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	if node.Endpoints != nil {
		delete(node.Endpoints, endpointID)
	}

	s.logger.Debug(
		"node delete state; pending node",
		zap.String("node-id", nodeID),
		zap.String("key", key),
	)
}

func (s *syncer) onLocalEndpointUpdate(endpointID string, listeners int) {
	key := "endpoint:" + endpointID
	if listeners > 0 {
		s.gossiper.UpsertLocal(key, strconv.Itoa(listeners))
	} else {
		s.gossiper.DeleteLocal(key)
	}
}

var _ kite.Watcher = &syncer{}
