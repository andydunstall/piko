package gossip

import "github.com/andydunstall/pico/serverv2/netmap"

// Gossip is responsible for maintaining the nodes local NetworkMap and
// propagating the state of the local node to the rest of the cluster.
//
// It uses the 'kite' library for cluster membership anti-entropy, where each
// node maintains a local key-value store containing the nodes state which is
// then propagated to the other nodes in the cluster. Therefore Gossip
// manages updating the local key-value for this node, and watching for updates
// to other nodes and adding them to the netmap.
type Gossip struct {
}

func NewGossip(_ *netmap.NetworkMap) (*Gossip, error) {
	return &Gossip{}, nil
}

func (g *Gossip) Join(_ []string) error {
	return nil
}

func (g *Gossip) Leave() error {
	return nil
}

func (g *Gossip) Close() error {
	return nil
}
