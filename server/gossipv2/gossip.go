package gossip

import (
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	netmap "github.com/andydunstall/pico/server/netmapv2"
)

type Gossip struct {
	networkMap *netmap.NetworkMap
	conf       *config.Config
	logger     log.Logger
}

func NewGossip(
	networkMap *netmap.NetworkMap,
	conf *config.Config,
	logger log.Logger,
) (*Gossip, error) {
	return &Gossip{
		networkMap: networkMap,
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
func (g *Gossip) NodesMetadata() []*NodeMetadata {
	return nil
}

// NodeState returns the known state of the node with the given ID.
func (g *Gossip) NodeState(_ string) (*NodeState, bool) {
	return nil, false
}

// LocalNodeState returns the known state of the local node.
func (g *Gossip) LocalNodeState() *NodeState {
	return nil
}

func (g *Gossip) Close() error {
	return nil
}
