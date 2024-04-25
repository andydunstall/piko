package gossip

import (
	"fmt"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	netmap "github.com/andydunstall/pico/server/netmapv2"
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

	conf *config.Config

	logger log.Logger
}

func NewGossip(
	networkMap *netmap.NetworkMap,
	conf *config.Config,
	logger log.Logger,
) (*Gossip, error) {

	syncer := newSyncer(networkMap, logger)
	gossiper, err := kite.New(
		kite.WithMemberID(networkMap.LocalNode().ID),
		kite.WithBindAddr(conf.Gossip.BindAddr),
		kite.WithAdvertiseAddr(conf.Gossip.AdvertiseAddr),
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
		logger:     logger.WithSubsystem("gossip"),
	}, nil
}

// Join attempts to join an existing cluster by syncronising with the members
// at the given addresses.
func (g *Gossip) Join(_ []string) ([]string, error) {
	return nil, nil
}

// Leave notifies the known members that this node is leaving the cluster.
//
// This will attempt to sync with up to 3 nodes to ensure the leave status is
// propagated.
//
// Returns an error if no known members could be notified.
func (g *Gossip) Leave() error {
	return nil
}

// NodesMetadata returns the metadata of all known nodes in the cluster.
func (g *Gossip) NodesMetadata() []kite.MemberMeta {
	return nil
}

// NodeState returns the known state of the node with the given ID.
func (g *Gossip) NodeState(_ string) (kite.MemberState, bool) {
	return kite.MemberState{}, false
}

// LocalNodeState returns the known state of the local node.
func (g *Gossip) LocalNodeState() kite.MemberState {
	return kite.MemberState{}
}

func (g *Gossip) Close() error {
	return nil
}
