package gossip

import (
	"context"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/netmap"
)

// Gossip is responsible for maintaining the nodes local NetworkMap and
// propagating the state of the local node to the rest of the cluster.
type Gossip struct {
	netmap *netmap.NetworkMap
	logger *log.Logger
}

// NewGossip initializes gossip to maintain the given network map.
func NewGossip(netmap *netmap.NetworkMap, logger *log.Logger) *Gossip {
	return &Gossip{
		netmap: netmap,
		logger: logger.WithSubsystem("gossip"),
	}
}

// Run gossips with the other nodes in the cluster until cancelled.
func (g *Gossip) Run(_ context.Context) error {
	g.logger.Info("starting gossip")
	return nil
}
