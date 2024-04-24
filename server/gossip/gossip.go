package gossip

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// Gossip is responsible for maintaining the nodes local NetworkMap and
// propagating the state of the local node to the rest of the cluster.
//
// It uses the 'kite' library for cluster membership anti-entropy, where each
// node maintains a local key-value store containing the nodes state which is
// then propagated to the other nodes in the cluster. Therefore Gossip
// manages updating the local key-value for this node, and watching for updates
// to other nodes and adding them to the netmap.
type Gossip struct {
	networkMap *netmap.NetworkMap

	// pendingNodes contains nodes that we haven't received the full state for
	// yet so can't be added to the netmap. Since kite propagates key-value
	// pairs in order, we may only have a few entries of a node. Therefore
	// a node is only considered 'complete' once we have its status (which is
	// always sent last).
	pendingNodes   map[string]*netmap.Node
	pendingNodesMu sync.Mutex

	kite *kite.Kite

	logger log.Logger
}

func NewGossip(
	bindAddr string,
	networkMap *netmap.NetworkMap,
	registry *prometheus.Registry,
	logger log.Logger,
) (*Gossip, error) {
	gossip := &Gossip{
		networkMap:   networkMap,
		pendingNodes: make(map[string]*netmap.Node),
		logger:       logger.WithSubsystem("gossip"),
	}

	networkMap.OnLocalUpsert(gossip.onLocalUpsert)

	kite, err := kite.New(
		kite.WithMemberID(networkMap.LocalNode().ID),
		kite.WithBindAddr(bindAddr),
		kite.WithAdvertiseAddr(networkMap.LocalNode().GossipAddr),
		kite.WithWatcher(newKiteWatcher(gossip)),
		kite.WithPrometeusRegistry(registry),
		kite.WithLogger(logger.WithSubsystem("gossip.kite")),
	)
	if err != nil {
		return nil, fmt.Errorf("kite: %w", err)
	}
	gossip.kite = kite

	gossip.updateLocalState()

	return gossip, nil
}

func (g *Gossip) Join(addrs []string) error {
	_, err := g.kite.Join(addrs)
	return err
}

func (g *Gossip) Leave(ctx context.Context) error {
	ch := make(chan error, 1)
	go func() {
		ch <- g.kite.Leave()
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (g *Gossip) MembersMetadata(filter kite.MemberFilter) []kite.MemberMeta {
	return g.kite.MembersMetadata(filter)
}

func (g *Gossip) MemberState(id string) (kite.MemberState, bool) {
	return g.kite.MemberState(id)
}

func (g *Gossip) Close() error {
	return g.kite.Close()
}

// updateLocalState updates the local Kite key-value state which will be
// propagated to other nodes.
func (g *Gossip) updateLocalState() {
	localNode := g.networkMap.LocalNode()
	g.kite.UpsertLocal("proxy_addr", localNode.ProxyAddr)
	g.kite.UpsertLocal("admin_addr", localNode.AdminAddr)
	g.kite.UpsertLocal("gossip_addr", localNode.GossipAddr)
	// Note adding the status last since a node is considered 'pending' until
	// the status is known.
	g.kite.UpsertLocal("status", string(localNode.Status))
}

func (g *Gossip) onLocalUpsert(key, value string) {
	if value == "" {
		g.kite.DeleteLocal(key)
	} else {
		g.kite.UpsertLocal(key, value)
	}
}

func (g *Gossip) onRemoteJoin(nodeID string) {
	if _, ok := g.networkMap.Node(nodeID); ok {
		g.logger.Warn(
			"node joined; already in netmap",
			zap.String("node-id", nodeID),
		)
		return
	}

	g.pendingNodesMu.Lock()
	defer g.pendingNodesMu.Unlock()

	if _, ok := g.pendingNodes[nodeID]; ok {
		g.logger.Warn(
			"node joined; already pending",
			zap.String("node-id", nodeID),
		)
		return
	}

	// Add as pending since we don't have enough information to add to the
	// netmap.
	g.pendingNodes[nodeID] = &netmap.Node{
		ID: nodeID,
	}

	g.logger.Info("node joined", zap.String("node-id", nodeID))
}

func (g *Gossip) onRemoteLeave(nodeID string) {
	if deleted := g.networkMap.RemoveRemote(nodeID); deleted {
		g.logger.Info(
			"node left; removed from netmap",
			zap.String("node-id", nodeID),
		)
		return
	}

	g.pendingNodesMu.Lock()
	defer g.pendingNodesMu.Unlock()

	_, ok := g.pendingNodes[nodeID]
	if ok {
		delete(g.pendingNodes, nodeID)

		g.logger.Info(
			"node left; removed from pending",
			zap.String("node-id", nodeID),
		)
	} else {
		g.logger.Warn(
			"node left; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (g *Gossip) onRemoteDown(nodeID string) {
	g.logger.Info("node down", zap.String("node-id", nodeID))

	// TODO(andydunstall): Need to notify if it comes back up as well.
}

func (g *Gossip) onRemoteUpsert(nodeID, key, value string) {
	if g.networkMap.UpsertRemote(nodeID, key, value) {
		g.logger.Debug(
			"node updated; netmap updated",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
		return
	}

	g.pendingNodesMu.Lock()
	defer g.pendingNodesMu.Unlock()

	node, ok := g.pendingNodes[nodeID]
	if !ok {
		g.logger.Warn(
			"node updated; unknown node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
		return
	}

	switch key {
	case "proxy_addr":
		node.ProxyAddr = value
	case "admin_addr":
		node.AdminAddr = value
	case "gossip_addr":
		node.GossipAddr = value
	case "status":
		node.Status = netmap.NodeStatus(value)
	default:
		if strings.HasPrefix(key, "endpoint:") {
			endpointID, _ := strings.CutPrefix(key, "endpoint:")
			numListeners, err := strconv.Atoi(value)
			if err != nil {
				g.logger.Error(
					"invalid endpoint: num listeners",
					zap.String("node-id", node.ID),
					zap.Error(err),
				)
			}
			if node.Endpoints == nil {
				node.Endpoints = make(map[string]int)
			}
			node.Endpoints[endpointID] = numListeners
		} else {
			g.logger.Error(
				"unknown key",
				zap.String("node-id", node.ID),
				zap.String("key", key),
			)
		}

		return
	}

	// Once we have the node status for the pending node, it can be added to
	// the netmap.
	if node.Status != "" {
		delete(g.pendingNodes, node.ID)
		g.networkMap.AddRemote(node)

		g.logger.Debug(
			"node updated; added to netmap",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
	} else {
		g.logger.Debug(
			"node updated; pending node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
			zap.String("value", value),
		)
	}
}

func (g *Gossip) onRemoteDelete(nodeID, key string) {
	if g.networkMap.DeleteRemoteState(nodeID, key) {
		g.logger.Debug(
			"node key deleted; netmap updated",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	g.pendingNodesMu.Lock()
	defer g.pendingNodesMu.Unlock()

	node, ok := g.pendingNodes[nodeID]
	if !ok {
		g.logger.Warn(
			"node key deleted; unknown node",
			zap.String("node-id", nodeID),
			zap.String("key", key),
		)
		return
	}

	if strings.HasPrefix(key, "endpoint:") {
		endpointID, _ := strings.CutPrefix(key, "endpoint:")
		if node.Endpoints != nil {
			delete(node.Endpoints, endpointID)
		}
	} else {
		g.logger.Error(
			"unknown key",
			zap.String("node-id", node.ID),
			zap.String("key", key),
		)
	}

	g.logger.Debug(
		"node key deleted; pending node",
		zap.String("node-id", nodeID),
		zap.String("key", key),
	)
}

// kiteWatcher is a kite.Watcher which is notified when nodes in the cluster
// are updated.
type kiteWatcher struct {
	gossip *Gossip
}

func newKiteWatcher(gossip *Gossip) *kiteWatcher {
	return &kiteWatcher{
		gossip: gossip,
	}
}

func (w *kiteWatcher) OnJoin(memberID string) {
	w.gossip.onRemoteJoin(memberID)
}

func (w *kiteWatcher) OnLeave(memberID string) {
	w.gossip.onRemoteLeave(memberID)
}

func (w *kiteWatcher) OnHealthy(_ string) {
	// TODO(andydunstall)
}

func (w *kiteWatcher) OnDown(memberID string) {
	w.gossip.onRemoteDown(memberID)
}

func (w *kiteWatcher) OnUpsert(memberID, key, value string) {
	w.gossip.onRemoteUpsert(memberID, key, value)
}

func (w *kiteWatcher) OnDelete(memberID, key string) {
	w.gossip.onRemoteDelete(memberID, key)
}

var _ kite.Watcher = &kiteWatcher{}
