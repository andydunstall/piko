package gossip

import (
	"context"
	"net"
	"time"

	"go.uber.org/zap"

	"github.com/dragonflydb/piko/pkg/backoff"
	"github.com/dragonflydb/piko/pkg/gossip"
	"github.com/dragonflydb/piko/pkg/log"
	"github.com/dragonflydb/piko/server/cluster"
)

// Gossip is responsible for maintaining this nodes local State
// and propagating the state of the local node to the rest of the cluster.
//
// At the gossip layer, a nodes state is represented as key-value pairs which
// are propagated around the cluster. These key-value pairs are then
// used to gossip based anti-entropy protocol. These key-value pairs are then
// used to build the local State.
type Gossip struct {
	clusterState *cluster.State

	// gossiper manages communicating with the other members to exchange state
	// updates.
	gossiper *gossip.Gossip

	logger log.Logger
}

func NewGossip(
	clusterState *cluster.State,
	streamLn net.Listener,
	packetLn net.PacketConn,
	conf *gossip.Config,
	logger log.Logger,
) *Gossip {
	logger = logger.WithSubsystem("gossip")

	syncer := newSyncer(clusterState, logger)
	gossiper := gossip.New(
		clusterState.LocalNode().ID,
		conf,
		streamLn,
		packetLn,
		syncer,
		logger,
	)
	syncer.Sync(gossiper)

	return &Gossip{
		clusterState: clusterState,
		gossiper:     gossiper,
		logger:       logger,
	}
}

// JoinOnBoot attempts to join an existing cluster by syncronising with the
// members at the given addresses.
//
// This will only attempt to join once and won't retry.
func (g *Gossip) JoinOnBoot(addrs []string) ([]string, error) {
	return g.gossiper.Join(addrs)
}

// JoinOnStartup attempts to join an existing cluster by syncronising with the
// members at the given addresses.
//
// This will retry 5 times (with backoff).
func (g *Gossip) JoinOnStartup(ctx context.Context, addrs []string) ([]string, error) {
	backoff := backoff.New(5, time.Second, time.Minute)
	var lastErr error
	for {
		nodeIDs, err := g.gossiper.Join(addrs)
		if err == nil {
			return nodeIDs, nil
		}

		backoff, retry := backoff.Backoff()
		if !retry {
			return nil, lastErr
		}

		g.logger.Warn(
			"failed to join cluster; retrying",
			zap.String("backoff", backoff.String()),
			zap.Error(err),
		)
		lastErr = err

		select {
		case <-time.After(backoff):
			continue
		case <-ctx.Done():
			return nil, lastErr
		}
	}
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
func (g *Gossip) Nodes() []gossip.NodeMetadata {
	return g.gossiper.Nodes()
}

// NodeState returns the known state of the node with the given ID.
func (g *Gossip) NodeState(id string) (*gossip.NodeState, bool) {
	return g.gossiper.Node(id)
}

func (g *Gossip) Metrics() *gossip.Metrics {
	return g.gossiper.Metrics()
}

func (g *Gossip) Close() error {
	return g.gossiper.Close()
}
