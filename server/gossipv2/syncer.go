package gossip

import (
	"strconv"
	"strings"
	"sync"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/pkg/log"
	netmap "github.com/andydunstall/pico/server/netmapv2"
	"go.uber.org/zap"
)

type gossiper interface {
	UpsertLocal(key, value string)
	DeleteLocal(key string)
}

// syncer handles syncronising state between gossip and the netmap.
//
// When a node joins, it is considered 'pending' so not added to the netmap
// until we have the full node state. Since gossip propagates state updates in
// order, we always send the initial node state status last, meaning if we
// have the status of a remote node, we'll have its other immutable fields.
type syncer struct {
	// pendingNodes contains nodes that we haven't received the full state for
	// yet so can't be added to the netmap.
	pendingNodes map[string]*netmap.Node

	// mu protects the above fields.
	mu sync.Mutex

	networkMap *netmap.NetworkMap

	gossiper gossiper

	logger log.Logger
}

func newSyncer(networkMap *netmap.NetworkMap, logger log.Logger) *syncer {
	return &syncer{
		pendingNodes: make(map[string]*netmap.Node),
		networkMap:   networkMap,
		logger:       logger,
	}
}

func (s *syncer) Sync(gossiper gossiper) {
	s.gossiper = gossiper

	s.networkMap.OnLocalStatusUpdate(s.onLocalStatusUpdate)
	s.networkMap.OnLocalEndpointUpdate(s.onLocalEndpointUpdate)

	localNode := s.networkMap.LocalNode()
	// First add immutable fields.
	s.gossiper.UpsertLocal("proxy_addr", localNode.ProxyAddr)
	s.gossiper.UpsertLocal("admin_addr", localNode.AdminAddr)
	// Next add status, which means receiving nodes will consider the node
	// state 'complete' so add to the netmap.
	s.gossiper.UpsertLocal("status", string(localNode.Status))
	// Finally add mutable fields.
	for endpointID, listeners := range localNode.Endpoints {
		key := "endpoint:" + endpointID
		s.gossiper.UpsertLocal(key, strconv.Itoa(listeners))
	}
}

func (s *syncer) OnJoin(nodeID string) {
	if nodeID == s.networkMap.LocalID() {
		s.logger.Warn(
			"node joined; same id as local node",
			zap.String("node-id", nodeID),
		)
		return
	}

	if _, ok := s.networkMap.Node(nodeID); ok {
		s.logger.Warn(
			"node joined; already in netmap",
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
	// netmap.
	s.pendingNodes[nodeID] = &netmap.Node{
		ID: nodeID,
	}

	s.logger.Info("node joined", zap.String("node-id", nodeID))
}

func (s *syncer) OnLeave(nodeID string) {
	if deleted := s.networkMap.RemoveNode(nodeID); deleted {
		s.logger.Info(
			"node leave; removed from netmap",
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

func (s *syncer) OnHealthy(_ string) {
	// TODO(andydunstall):
	// If a node goes down then comes back, need to ensure we still have the
	// state of that node. Therefore if a node is down, mark it as down in the
	// netmap but don't remove. Wait for Kite to send OnLeave before actually
	// removing from the netmap (since we know Kite will do an OnJoin if it
	// comes back). Therefore need to update Kite.
}

func (s *syncer) OnDown(_ string) {
	// TODO(andydunstall): See above.
}

func (s *syncer) OnUpsert(nodeID, key, value string) {
	if nodeID == s.networkMap.LocalID() {
		s.logger.Warn(
			"node upsert state; same id as local node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	// First check if the node is already in the netmap. Only check mutable
	// fields.
	if key == "status" {
		if s.networkMap.UpdateRemoteStatus(nodeID, netmap.NodeStatus(value)) {
			return
		}
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
		if s.networkMap.UpdateRemoteEndpoint(nodeID, endpointID, listeners) {
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
	}

	if key == "status" {
		node.Status = netmap.NodeStatus(value)
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

	// Once we have the node status for the pending node, it can be added to
	// the netmap.
	if node.Status != "" {
		delete(s.pendingNodes, node.ID)
		s.networkMap.AddNode(node)

		s.logger.Debug(
			"node upsert state; added to netmap",
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

func (s *syncer) OnDelete(nodeID, key string) {
	if nodeID == s.networkMap.LocalID() {
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
	if s.networkMap.RemoveRemoteEndpoint(nodeID, endpointID) {
		s.logger.Debug(
			"node delete state; netmap updated",
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

func (s *syncer) onLocalStatusUpdate(status netmap.NodeStatus) {
	s.gossiper.UpsertLocal("status", string(status))
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
