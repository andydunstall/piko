package gossip

import (
	"context"
	"fmt"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
)

// Gossip is responsible for maintaining this nodes local NetworkMap
// and propagating the state of the local node to the rest of the cluster.
//
// At the gossip layer, a nodes state is represented as key-value pairs which
// are propagated around the cluster using Kite. These key-value pairs are then
// used to gossip based anti-entropy protocol. These key-value pairs are then
// used to build the local NetworkMap.
type Gossip struct {
	networkMap *netmap.NetworkMap

	// gossiper manages communicating with the other members to exchange state
	// updates.
	gossiper *kite.Kite

	conf Config

	logger log.Logger
}

func NewGossip(
	networkMap *netmap.NetworkMap,
	conf Config,
	logger log.Logger,
) (*Gossip, error) {
	logger = logger.WithSubsystem("gossip")

	syncer := newSyncer(networkMap, logger)
	gossiper, err := kite.New(
		kite.WithNodeID(networkMap.LocalNode().ID),
		kite.WithBindAddr(conf.BindAddr),
		kite.WithAdvertiseAddr(conf.AdvertiseAddr),
		kite.WithWatcher(syncer),
		kite.WithLogger(logger.WithSubsystem("gossip.kite")),
	)
	if err != nil {
		return nil, fmt.Errorf("kite: %w", err)
	}
	syncer.Sync(gossiper)

	return &Gossip{
		networkMap: networkMap,
		gossiper:   gossiper,
		conf:       conf,
		logger:     logger,
	}, nil
}

// Join attempts to join an existing cluster by syncronising with the members
// at the given addresses.
func (g *Gossip) Join(addrs []string) ([]string, error) {
	return g.gossiper.Join(addrs)
}

// Leave notifies the known members that this node is leaving the cluster.
//
// This will attempt to sync with up to 3 nodes to ensure the leave status is
// propagated.
//
// Returns an error if no known members could be notified.
func (g *Gossip) Leave(ctx context.Context) error {
	ch := make(chan error, 1)
	go func() {
		ch <- g.gossiper.Leave()
	}()

	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Nodes returns the metadata of all known nodes in the cluster.
func (g *Gossip) Nodes() []kite.NodeMetadata {
	return g.gossiper.Nodes()
}

// NodeState returns the known state of the node with the given ID.
func (g *Gossip) NodeState(id string) (*kite.NodeState, bool) {
	return g.gossiper.Node(id)
}

func (g *Gossip) Close() error {
	return g.gossiper.Close()
}
