//go:build integration

package gossip

import (
	"context"
	"fmt"
	"net"
	"sort"
	"testing"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGossip_Join(t *testing.T) {
	t.Run("single node by ip", func(t *testing.T) {
		node1 := testNode("node-1", t)
		defer node1.Close()

		node1.UpsertLocal("k1", "v1")
		node1.UpsertLocal("k2", "v2")
		node1.UpsertLocal("k3", "v3")

		node2 := testNode("node-2", t)
		defer node2.Close()

		node2.UpsertLocal("k1", "v1")
		node2.UpsertLocal("k2", "v2")
		node2.UpsertLocal("k3", "v3")

		nodeIDs, err := node2.Join([]string{node1.LocalNode().Addr})
		require.NoError(t, err)
		assert.Equal(t, []string{"node-1"}, nodeIDs)

		// Verify each node discovered the other.
		assertNodesEqual := func(node *Gossip) {
			nodes := node.Nodes()
			// Sort by node ID.
			sort.Slice(nodes, func(i, j int) bool {
				return nodes[i].ID < nodes[j].ID
			})
			assert.Equal(
				t,
				[]NodeMetadata{
					{"node-1", node1.LocalNode().Addr, uint64(3), false, false, time.Time{}},
					{"node-2", node2.LocalNode().Addr, uint64(3), false, false, time.Time{}},
				},
				nodes,
			)
		}

		assertNodesEqual(node1)
		assertNodesEqual(node2)
	})

	t.Run("addr unreachable", func(t *testing.T) {
		node := testNode("node-1", t)
		defer node.Close()

		_, err := node.Join([]string{"127.1.1.1"})
		assert.Error(t, err)
	})
}

func TestGossip_Leave(t *testing.T) {
	t.Run("leave single node", func(t *testing.T) {
		node1 := testNode("node-1", t)
		defer node1.Close()

		node2 := testNode("node-2", t)
		defer node2.Close()

		nodeIDs, err := node2.Join([]string{node1.LocalNode().Addr})
		require.NoError(t, err)
		assert.Equal(t, []string{"node-1"}, nodeIDs)

		assert.NoError(t, node2.Leave())

		// Verify each node now considers the node as having left.
		assertNodeLeft := func(node *Gossip) {
			state, _ := node.Node("node-1")
			assert.False(t, state.Left)

			state, _ = node.Node("node-2")
			assert.True(t, state.Left)
		}

		assertNodeLeft(node1)
		assertNodeLeft(node2)
	})

	t.Run("known node unreachable", func(t *testing.T) {
		node1 := testNode("node-1", t)
		defer node1.Close()

		node2 := testNode("node-2", t)
		defer node2.Close()

		nodeIDs, err := node2.Join([]string{node1.LocalNode().Addr})
		require.NoError(t, err)
		assert.Equal(t, []string{"node-1"}, nodeIDs)

		// Close the only known node prior to leaving.
		node1.Close()

		assert.Error(t, node2.Leave())
	})
}

func TestGossip_Gossip(t *testing.T) {
	t.Run("propagate update", func(t *testing.T) {
		node1Watcher := &updateWatcher{
			Ch: make(chan updateEvent, 10),
		}
		defer node1Watcher.Close()

		node1 := testNodeWithWatcher("node-1", node1Watcher, t)
		defer node1.Close()

		node2 := testNodeWithWatcher("node-2", node1Watcher, t)
		defer node2.Close()

		_, err := node2.Join([]string{node1.LocalNode().Addr})
		require.NoError(t, err)

		// Update node 2 and wait for the update to be propagated to node 1.
		for i := 0; i != 10; i++ {
			node2.UpsertLocal(
				fmt.Sprintf("key-%d", i),
				fmt.Sprintf("value-%d", i),
			)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()

		for i := 0; i != 10; i++ {
			event, err := node1Watcher.Next(ctx)
			assert.NoError(t, err)
			assert.Equal(t, updateEvent{
				NodeID: "node-2",
				Key:    fmt.Sprintf("key-%d", i),
				Value:  fmt.Sprintf("value-%d", i),
			}, event)
		}
	})
}

func TestGossip_NodeUnreachable(t *testing.T) {
	t.Run("detect unreachable", func(t *testing.T) {
		node1Watcher := &livenessWatcher{
			Ch: make(chan livenessEvent, 10),
		}
		defer node1Watcher.Close()

		node1 := testNodeWithWatcher("node-1", node1Watcher, t)
		defer node1.Close()

		node2 := testNode("node-2", t)
		defer node2.Close()

		_, err := node2.Join([]string{node1.LocalNode().Addr})
		require.NoError(t, err)

		// Close without leaving gracefully.
		node2.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		event, err := node1Watcher.Next(ctx)
		assert.NoError(t, err)
		assert.Equal(t, livenessEvent{
			NodeID:      "node-2",
			Unreachable: true,
		}, event)
	})
}

type updateEvent struct {
	NodeID string
	Key    string
	Value  string
}

type updateWatcher struct {
	Ch chan updateEvent
}

func (w *updateWatcher) OnJoin(_ string) {}

func (w *updateWatcher) OnLeave(_ string) {}

func (w *updateWatcher) OnReachable(_ string) {}

func (w *updateWatcher) OnUnreachable(_ string) {}

func (w *updateWatcher) OnUpsertKey(nodeID, key, value string) {
	w.Ch <- updateEvent{
		NodeID: nodeID,
		Key:    key,
		Value:  value,
	}
}

func (w *updateWatcher) OnDeleteKey(nodeID, key string) {
	w.Ch <- updateEvent{
		NodeID: nodeID,
		Key:    key,
	}
}

func (w *updateWatcher) OnExpired(_ string) {}

func (w *updateWatcher) Next(ctx context.Context) (updateEvent, error) {
	select {
	case event := <-w.Ch:
		return event, nil
	case <-ctx.Done():
		return updateEvent{}, ctx.Err()
	}
}

func (w *updateWatcher) Close() {
	close(w.Ch)
}

var _ Watcher = &updateWatcher{}

type livenessEvent struct {
	NodeID      string
	Unreachable bool
}

type livenessWatcher struct {
	Ch chan livenessEvent
}

func (w *livenessWatcher) OnJoin(_ string) {}

func (w *livenessWatcher) OnLeave(_ string) {}

func (w *livenessWatcher) OnReachable(nodeID string) {
	w.Ch <- livenessEvent{
		NodeID:      nodeID,
		Unreachable: false,
	}
}

func (w *livenessWatcher) OnUnreachable(nodeID string) {
	w.Ch <- livenessEvent{
		NodeID:      nodeID,
		Unreachable: true,
	}
}

func (w *livenessWatcher) OnUpsertKey(_, _, _ string) {}

func (w *livenessWatcher) OnDeleteKey(_, _ string) {}

func (w *livenessWatcher) OnExpired(_ string) {}

func (w *livenessWatcher) Next(ctx context.Context) (livenessEvent, error) {
	select {
	case event := <-w.Ch:
		return event, nil
	case <-ctx.Done():
		return livenessEvent{}, ctx.Err()
	}
}

func (w *livenessWatcher) Close() {
	close(w.Ch)
}

func testNode(nodeID string, t *testing.T) *Gossip {
	return testNodeWithWatcher(nodeID, newNopWatcher(), t)
}

func testNodeWithWatcher(nodeID string, w Watcher, t *testing.T) *Gossip {
	streamLn, packetLn := testListen(t)
	nodeConfig := testConfig()
	nodeConfig.AdvertiseAddr = streamLn.Addr().String()
	return New(
		nodeID,
		nodeConfig,
		streamLn,
		packetLn,
		w,
		log.NewNopLogger(),
	)
}

func testListen(t *testing.T) (net.Listener, net.PacketConn) {
	streamLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	packetLn, err := net.ListenUDP("udp", &net.UDPAddr{
		IP:   streamLn.Addr().(*net.TCPAddr).IP,
		Port: streamLn.Addr().(*net.TCPAddr).Port,
	})
	require.NoError(t, err)

	return streamLn, packetLn
}

func testConfig() *Config {
	return &Config{
		BindAddr:      "127.0.0.1:0",
		Interval:      time.Millisecond * 10,
		MaxPacketSize: 1400,
	}
}
