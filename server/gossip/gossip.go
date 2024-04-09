package gossip

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
	"github.com/andydunstall/seal"
	"go.uber.org/zap"
)

// Gossip is responsible for maintaining the nodes local NetworkMap and
// propagating the state of the local node to the rest of the cluster.
//
// It uses the 'seal' library for cluster membership anti-entropy, where each
// node maintains a local key-value store containing the nodes state which is
// then propagated to the other nodes in the cluster. Therefore Gossip
// manages updating the local key-value for this node, and watching for updates
// to other nodes and adding them to the netmap.
type Gossip struct {
	networkMap *netmap.NetworkMap

	seal *seal.Seal

	// pendingNodes contains nodes that we haven't received the full state for
	// yet so can't be added to the netmap. Since seal propagates key-value
	// pairs in order, we may only have a few entries of a node. Therefore
	// a node is only considered 'complete' once we have its status (which is
	// always sent last).
	pendingNodes   map[string]*netmap.Node
	pendingNodesMu sync.Mutex

	logger *log.Logger
}

func NewGossip(networkMap *netmap.NetworkMap, logger *log.Logger) (*Gossip, error) {
	gossip := &Gossip{
		networkMap:   networkMap,
		pendingNodes: make(map[string]*netmap.Node),
		logger:       logger.WithSubsystem("gossip"),
	}

	seal, err := seal.New(
		networkMap.LocalNode().ID,
		seal.WithWatcher(newSealWatcher(gossip)),
		seal.WithLogger(logger.WithSubsystem("gossip.seal")),
	)
	if err != nil {
		return nil, fmt.Errorf("seal: %w", err)
	}
	gossip.seal = seal
	gossip.updateLocalState()

	networkMap.OnLocalStatusUpdated(gossip.onLocalStatusUpdated)
	networkMap.OnLocalEndpointUpdated(gossip.onLocalEndpointUpdated)
	networkMap.OnLocalEndpointRemoved(gossip.onLocalEndpointRemoved)

	return gossip, nil
}

func (g *Gossip) Join(ctx context.Context, addrs []string) error {
	return g.seal.Join(ctx, addrs)
}

func (g *Gossip) Leave(ctx context.Context) error {
	return g.seal.Leave(ctx)
}

func (g *Gossip) Close() error {
	return g.seal.Close()
}

func (g *Gossip) onLocalStatusUpdated(status netmap.NodeStatus) {
	g.seal.UpsertLocal("status", string(status))
}

func (g *Gossip) onLocalEndpointUpdated(endpointID string, numListeners int) {
	if numListeners > 0 {
		g.seal.UpsertLocal("endpoint:"+endpointID, strconv.Itoa(numListeners))
	} else {
		g.seal.DeleteLocal("endpoint:" + endpointID)
	}
}

func (g *Gossip) onLocalEndpointRemoved(endpointID string) {
	g.seal.DeleteLocal("endpoint:" + endpointID)
}

// updateLocalState updates the local Seal key-value state which will be
// propagated to other nodes.
func (g *Gossip) updateLocalState() {
	localNode := g.networkMap.LocalNode()
	g.seal.UpsertLocal("http_addr", localNode.HTTPAddr)
	g.seal.UpsertLocal("gossip_addr", localNode.GossipAddr)
	for endpointID, numListeners := range localNode.Endpoints {
		if numListeners > 0 {
			g.seal.UpsertLocal("endpoint:"+endpointID, strconv.Itoa(numListeners))
		}
	}
	// Note adding the status last since a node is considered 'pending' until
	// the status is known.
	g.seal.UpsertLocal("status", string(localNode.Status))
}

func (g *Gossip) onRemoteJoin(nodeID string) {
	if _, ok := g.networkMap.NodeByID(nodeID); ok {
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

	g.logger.Info(
		"node joined",
		zap.String("node-id", nodeID),
	)
	// Add as pending since we don't have enough information to add to the
	// netmap.
	g.pendingNodes[nodeID] = &netmap.Node{
		ID: nodeID,
	}
}

func (g *Gossip) onRemoteLeave(nodeID string) {
	if deleted := g.networkMap.DeleteNodeByID(nodeID); deleted {
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
	if deleted := g.networkMap.DeleteNodeByID(nodeID); deleted {
		g.logger.Info(
			"node down; removed from netmap",
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
			"node down; removed from pending",
			zap.String("node-id", nodeID),
		)
	} else {
		g.logger.Warn(
			"node down; unknown node",
			zap.String("node-id", nodeID),
		)
	}
}

func (g *Gossip) onRemoteUpdate(nodeID, key, value string) {
	if ok := g.networkMap.UpdateNodeByID(nodeID, func(n *netmap.Node) {
		g.applyNodeUpdate(n, key, value)
	}); ok {
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

	g.applyNodeUpdate(node, key, value)

	g.logger.Debug(
		"node updated; pending node",
		zap.String("node-id", nodeID),
		zap.String("key", key),
		zap.String("value", value),
	)

	// Once we have the node state for the pending node, it can be added to
	// the netmap.
	if node.Status != "" {
		delete(g.pendingNodes, node.ID)
		g.networkMap.AddNode(node)
	}
}

func (g *Gossip) applyNodeUpdate(node *netmap.Node, key, value string) {
	switch key {
	case "http_addr":
		node.HTTPAddr = value
	case "gossip_addr":
		node.GossipAddr = value
	case "status":
		node.Status = netmap.NodeStatus(value)
	default:
		if strings.HasPrefix(key, "endpoint:") {
			endpointID, _ := strings.CutPrefix(key, "endpoint:")
			numListeners, err := strconv.Atoi(value)
			if err != nil {
				g.logger.Warn(
					"invalid endpoint: num listeners",
					zap.String("node-id", node.ID),
					zap.Error(err),
				)
				return
			}
			node.Endpoints[endpointID] = numListeners
		} else {
			g.logger.Warn(
				"unknown key",
				zap.String("node-id", node.ID),
				zap.String("key", key),
			)
		}
	}
}

// sealWatcher is a seal.Watcher which is notified when nodes in the cluster
// are updated.
type sealWatcher struct {
	gossip *Gossip
}

func newSealWatcher(gossip *Gossip) *sealWatcher {
	return &sealWatcher{
		gossip: gossip,
	}
}

func (w *sealWatcher) OnJoin(memberID string) {
	w.gossip.onRemoteJoin(memberID)
}

func (w *sealWatcher) OnLeave(memberID string) {
	w.gossip.onRemoteLeave(memberID)
}

func (w *sealWatcher) OnDown(memberID string) {
	w.gossip.onRemoteDown(memberID)
}

func (w *sealWatcher) OnUpdate(memberID, key, value string) {
	w.gossip.onRemoteUpdate(memberID, key, value)
}

var _ seal.Watcher = &sealWatcher{}
