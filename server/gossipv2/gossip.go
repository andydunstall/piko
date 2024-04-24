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
//
// The addresses may be either IP addresses or domains. If a domain is used
// all resolved IPs will be joined. If the port is omitted the default bind
// port is used. Note if a domain is used and it doesn't resolve to any
// members (other then ourselves), join will succeed.
//
// The local node sends its full local member state to the target member along
// with a delta, then receives any cluster state the local node is missing from
// the remote member.
//
// If the address is a domain, gossip will periodically resolve the domain
// and attempt to gossip with any unknown nodes. This is used to resolve
// network splits and handle the domain entries being updated.
//
// Returns the IDs of the joined nodes.
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
