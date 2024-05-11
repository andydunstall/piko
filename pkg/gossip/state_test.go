package gossip

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestClusterState_LocalState(t *testing.T) {
	t.Run("initial state", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)
		node := clusterState.LocalNode()
		assert.Equal(t, "node-1", node.ID)
		assert.Equal(t, "1.1.1.1:1", node.Addr)
		assert.Equal(t, uint64(0), node.Version)
		assert.Equal(t, false, node.Left)
		assert.Equal(t, false, node.Unreachable)
		assert.Equal(t, 0, len(node.Entries))
	})

	t.Run("upsert", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)

		clusterState.UpsertLocal("k1", "v1")
		clusterState.UpsertLocal("k2", "v2")
		clusterState.UpsertLocal("k3", "v3")

		node := clusterState.LocalNode()
		assert.Equal(t, uint64(3), node.Version)
		assert.Equal(
			t,
			[]Entry{
				{"k1", "v1", 1, false, false},
				{"k2", "v2", 2, false, false},
				{"k3", "v3", 3, false, false},
			},
			node.Entries,
		)
	})

	t.Run("delete", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)

		clusterState.UpsertLocal("k1", "v1")
		clusterState.UpsertLocal("k2", "v2")
		clusterState.UpsertLocal("k3", "v3")
		clusterState.DeleteLocal("k1")
		clusterState.DeleteLocal("k2")

		node := clusterState.LocalNode()
		assert.Equal(t, uint64(5), node.Version)
		assert.Equal(
			t,
			[]Entry{
				{"k3", "v3", 3, false, false},
				{"k1", "", 4, false, true},
				{"k2", "", 5, false, true},
			},
			node.Entries,
		)
	})
}

func TestClusterState_ApplyDigest(t *testing.T) {
	t.Run("apply", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1", &fakeFailureDetector{}, newNopWatcher(),
		)

		clusterState.ApplyDigest(digest{
			{"node-2", "2.2.2.2", 5, false},
			{"node-3", "3.3.3.3", 12, false},
			{"node-4", "4.4.4.4", 2, false},
		})

		nodes := clusterState.Nodes()
		// Sort by node ID.
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})

		assert.Equal(
			t,
			[]NodeMetadata{
				{"node-1", "1.1.1.1", uint64(0), false, false, time.Time{}},
				{"node-2", "2.2.2.2", uint64(0), false, false, time.Time{}},
				{"node-3", "3.3.3.3", uint64(0), false, false, time.Time{}},
				{"node-4", "4.4.4.4", uint64(0), false, false, time.Time{}},
			},
			nodes,
		)
	})

	t.Run("ignore left", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1", &fakeFailureDetector{}, newNopWatcher(),
		)

		// Apply should ignore left nodes.
		clusterState.ApplyDigest(digest{
			{"node-2", "2.2.2.2", 5, true},
			{"node-3", "3.3.3.3", 12, true},
			{"node-4", "4.4.4.4", 2, false},
		})

		nodes := clusterState.Nodes()
		// Sort by node ID.
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})

		assert.Equal(
			t,
			[]NodeMetadata{
				{"node-1", "1.1.1.1", uint64(0), false, false, time.Time{}},
				{"node-4", "4.4.4.4", uint64(0), false, false, time.Time{}},
			},
			nodes,
		)
	})

	t.Run("watch", func(t *testing.T) {
		watcher := &fakeWatcher{}
		clusterState := newClusterState(
			"node-1", "1.1.1.1", &fakeFailureDetector{}, watcher,
		)

		clusterState.ApplyDigest(digest{
			{"node-2", "2.2.2.2", 5, false},
			{"node-3", "3.3.3.3", 12, true},
			{"node-4", "4.4.4.4", 2, false},
		})

		assert.Equal(t, []string{
			"node-2",
			"node-4",
		}, watcher.joins)
	})
}

func TestClusterState_ApplyDelta(t *testing.T) {
	t.Run("apply", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1", &fakeFailureDetector{}, newNopWatcher(),
		)

		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 8, false, false},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "v1", 8, false, false},
					{"k2", "v2", 12, false, false},
					{"k3", "v3", 13, false, false},
				},
			},
		})

		nodes := clusterState.Nodes()
		// Sort by node ID.
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})

		assert.Equal(
			t,
			[]NodeMetadata{
				{"node-1", "1.1.1.1", uint64(0), false, false, time.Time{}},
				{"node-2", "2.2.2.2", uint64(8), false, false, time.Time{}},
				{"node-3", "3.3.3.3", uint64(13), false, false, time.Time{}},
			},
			nodes,
		)

		state, _ := clusterState.Node("node-2")
		assert.Equal(t, &NodeState{
			NodeMetadata: NodeMetadata{
				ID:          "node-2",
				Addr:        "2.2.2.2",
				Version:     8,
				Left:        false,
				Unreachable: false,
				Expiry:      time.Time{},
			},
			Entries: []Entry{
				{"k1", "v1", 4, false, false},
				{"k2", "v2", 5, false, false},
				{"k3", "v3", 8, false, false},
			},
		}, state)

		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "", 14, false, true},
					{"k2", "", 16, false, true},
				},
			},
		})

		state, _ = clusterState.Node("node-2")
		assert.Equal(t, &NodeState{
			NodeMetadata: NodeMetadata{
				ID:          "node-2",
				Addr:        "2.2.2.2",
				Version:     16,
				Left:        false,
				Unreachable: false,
			},
			Entries: []Entry{
				{"k3", "v3", 8, false, false},
				{"k1", "", 14, false, true},
				{"k2", "", 16, false, true},
			},
		}, state)
	})

	t.Run("watch", func(t *testing.T) {
		watcher := &fakeWatcher{}
		clusterState := newClusterState(
			"node-1", "1.1.1.1", &fakeFailureDetector{}, watcher,
		)

		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 8, false, false},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "v1", 8, false, false},
					{"k2", "v2", 12, false, false},
					{"k3", "v3", 13, false, false},
				},
			},
		})
		// Delete keys.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "", 10, false, true},
				},
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
				Entries: []Entry{
					{"k1", "", 16, false, true},
					{"k3", "", 18, false, true},
				},
			},
		})

		assert.Equal(t, []string{"node-2", "node-3"}, watcher.joins)
		assert.Equal(t, []stateUpsert{
			{"node-2", "k1", "v1"},
			{"node-2", "k2", "v2"},
			{"node-2", "k3", "v3"},
			{"node-3", "k1", "v1"},
			{"node-3", "k2", "v2"},
			{"node-3", "k3", "v3"},
		}, watcher.upserts)
		assert.Equal(t, []stateDelete{
			{"node-2", "k1"},
			{"node-3", "k1"},
			{"node-3", "k3"},
		}, watcher.deletes)
	})
}

func TestClusterState_Digest(t *testing.T) {
	clusterState := newClusterState(
		"node-1", "1.1.1.1", &fakeFailureDetector{}, newNopWatcher(),
	)
	clusterState.UpsertLocal("k1", "v1")
	clusterState.UpsertLocal("k2", "v2")
	clusterState.UpsertLocal("k3", "v3")
	clusterState.DeleteLocal("k2")

	clusterState.ApplyDelta(delta{
		{
			ID:   "node-2",
			Addr: "2.2.2.2",
			Entries: []Entry{
				{"k1", "v1", 4, false, false},
				{"k2", "v2", 5, false, false},
				{"k3", "v3", 8, false, false},
			},
		},
		{
			ID:   "node-3",
			Addr: "3.3.3.3",
			Entries: []Entry{
				{"k1", "v1", 8, false, false},
				{"k2", "v2", 12, false, false},
				{"k3", "v3", 13, false, false},
			},
		},
	})

	stateDigest := clusterState.Digest()
	sort.Slice(stateDigest, func(i, j int) bool {
		return stateDigest[i].ID < stateDigest[j].ID
	})
	assert.Equal(t, digest{
		{"node-1", "1.1.1.1", 4, false},
		{"node-2", "2.2.2.2", 8, false},
		{"node-3", "3.3.3.3", 13, false},
	}, stateDigest)
}

func TestClusterState_Delta(t *testing.T) {
	clusterState := newClusterState(
		"node-1", "1.1.1.1", &fakeFailureDetector{}, newNopWatcher(),
	)
	clusterState.UpsertLocal("k1", "v1")
	clusterState.UpsertLocal("k2", "v2")
	clusterState.UpsertLocal("k3", "v3")
	clusterState.DeleteLocal("k2")

	clusterState.ApplyDelta(delta{
		{
			ID:   "node-2",
			Addr: "2.2.2.2",
			Entries: []Entry{
				{"k1", "v1", 4, false, false},
				{"k2", "v2", 5, false, false},
				{"k3", "v3", 8, false, false},
			},
		},
		{
			ID:   "node-3",
			Addr: "3.3.3.3",
			Entries: []Entry{
				{"k1", "v1", 8, false, false},
				{"k2", "v2", 12, false, false},
				{"k3", "v3", 13, false, false},
			},
		},
	})

	// Get a full delta.
	stateDelta := clusterState.Delta(digest{}, true)
	sort.Slice(stateDelta, func(i, j int) bool {
		return stateDelta[i].ID < stateDelta[j].ID
	})
	assert.Equal(t, delta{
		{
			ID:   "node-1",
			Addr: "1.1.1.1",
			Entries: []Entry{
				{"k1", "v1", 1, false, false},
				{"k3", "v3", 3, false, false},
				{"k2", "", 4, false, true},
			},
		},
		{
			ID:   "node-2",
			Addr: "2.2.2.2",
			Entries: []Entry{
				{"k1", "v1", 4, false, false},
				{"k2", "v2", 5, false, false},
				{"k3", "v3", 8, false, false},
			},
		},
		{
			ID:   "node-3",
			Addr: "3.3.3.3",
			Entries: []Entry{
				{"k1", "v1", 8, false, false},
				{"k2", "v2", 12, false, false},
				{"k3", "v3", 13, false, false},
			},
		},
	}, stateDelta)

	// Get an empty delta.
	stateDelta = clusterState.Delta(digest{}, false)
	assert.Equal(t, 0, len(stateDelta))

	// Get an partial delta.
	stateDelta = clusterState.Delta(digest{
		{"node-1", "1.1.1.1", 2, false},
		{"node-2", "2.2.2.2", 4, false},
	}, false)
	sort.Slice(stateDelta, func(i, j int) bool {
		return stateDelta[i].ID < stateDelta[j].ID
	})
	assert.Equal(t, delta{
		{
			ID:   "node-1",
			Addr: "1.1.1.1",
			Entries: []Entry{
				{"k3", "v3", 3, false, false},
				{"k2", "", 4, false, true},
			},
		},
		{
			ID:   "node-2",
			Addr: "2.2.2.2",
			Entries: []Entry{
				{"k2", "v2", 5, false, false},
				{"k3", "v3", 8, false, false},
			},
		},
	}, stateDelta)
}

func TestClusterState_Leave(t *testing.T) {
	t.Run("leave local", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)
		clusterState.LeaveLocal()

		node := clusterState.LocalNode()
		assert.Equal(t, "node-1", node.ID)
		assert.Equal(t, "1.1.1.1:1", node.Addr)
		assert.Equal(t, uint64(1), node.Version)
		assert.Equal(t, true, node.Left)
		assert.Equal(t, false, node.Unreachable)
		assert.Equal(
			t,
			[]Entry{
				{leftKey, "", 1, true, false},
			},
			node.Entries,
		)
	})

	t.Run("leave remote", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)

		// Add node-2.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
				},
			},
		})
		// Leave.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{leftKey, "", 6, true, false},
				},
			},
		})

		node, _ := clusterState.Node("node-2")
		assert.Equal(t, "node-2", node.ID)
		assert.Equal(t, true, node.Left)
		// Expiry should have been set.
		assert.NotEqual(t, time.Time{}, node.Expiry)
		assert.Equal(
			t,
			[]Entry{
				{"k1", "v1", 4, false, false},
				{"k2", "v2", 5, false, false},
				{leftKey, "", 6, true, false},
			},
			node.Entries,
		)
	})

	t.Run("watch", func(t *testing.T) {
		watcher := &fakeWatcher{}
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, watcher,
		)

		// Add node-2.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
				},
			},
		})
		// Leave.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{leftKey, "", 6, true, false},
				},
			},
		})

		assert.Equal(t, []string{"node-2"}, watcher.joins)
		assert.Equal(t, []string{"node-2"}, watcher.leaves)
	})

	t.Run("expire", func(t *testing.T) {
		watcher := &fakeWatcher{}
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, watcher,
		)

		// Add node-2.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
				},
			},
		})
		// Leave.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{leftKey, "", 6, true, false},
				},
			},
		})

		clusterState.RemoveExpiredAt(time.Now().Add(nodeExpiry * 2))

		assert.Equal(t, []string{"node-2"}, watcher.joins)
		assert.Equal(t, []string{"node-2"}, watcher.leaves)
		assert.Equal(t, []string{"node-2"}, watcher.expires)
	})
}

func TestClusterState_Compact(t *testing.T) {
	t.Run("compact local", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)

		clusterState.UpsertLocal("k1", "v1")
		clusterState.UpsertLocal("k2", "v2")
		clusterState.UpsertLocal("k3", "v3")
		clusterState.UpsertLocal("k4", "v4")
		clusterState.DeleteLocal("k2")
		clusterState.DeleteLocal("k3")

		// The number of deleted keys is less than the threshold so this should
		// do nothing.
		clusterState.CompactLocal(10)

		node := clusterState.LocalNode()
		assert.Equal(
			t,
			[]Entry{
				{"k1", "v1", 1, false, false},
				{"k4", "v4", 4, false, false},
				{"k2", "", 5, false, true},
				{"k3", "", 6, false, true},
			},
			node.Entries,
		)

		// The number of deleted keys is greater than the threshold so this
		// should compact.
		clusterState.CompactLocal(1)

		node = clusterState.LocalNode()
		assert.Equal(
			t,
			[]Entry{
				{"k1", "v1", 7, false, false},
				{"k4", "v4", 8, false, false},
				{compactKey, "6", 9, true, false},
			},
			node.Entries,
		)
	})

	t.Run("compact remote", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{}, newNopWatcher(),
		)

		// Add entries.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 4, false, false},
					{"k2", "v2", 5, false, false},
					{"k3", "v3", 6, false, false},
					{"k4", "v4", 7, false, false},
				},
			},
		})
		// Delete entries.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k2", "", 8, false, true},
					{"k3", "", 9, false, true},
				},
			},
		})
		// Compact entries.
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
				Entries: []Entry{
					{"k1", "v1", 10, false, false},
					{"k4", "v4", 11, false, false},
					{compactKey, "9", 12, true, false},
				},
			},
		})

		node, _ := clusterState.Node("node-2")
		assert.Equal(
			t,
			[]Entry{
				{"k1", "v1", 10, false, false},
				{"k4", "v4", 11, false, false},
				{compactKey, "9", 12, true, false},
			},
			node.Entries,
		)
	})
}

func TestClusterState_UpdateLiveness(t *testing.T) {
	t.Run("node unreachable", func(t *testing.T) {
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{
				map[string]float64{
					"node-2": 15.0,
					"node-3": 25.0,
				},
			}, newNopWatcher(),
		)
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
			},
		})

		clusterState.UpdateLiveness(20.0)

		node, _ := clusterState.Node("node-1")
		assert.False(t, node.Unreachable)
		node, _ = clusterState.Node("node-2")
		assert.False(t, node.Unreachable)
		node, _ = clusterState.Node("node-3")
		assert.True(t, node.Unreachable)
	})

	t.Run("node healthy", func(t *testing.T) {
		suspicionLevels := map[string]float64{
			"node-2": 15.0,
			"node-3": 25.0,
		}
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{suspicionLevels}, newNopWatcher(),
		)
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
			},
			{
				ID:   "node-3",
				Addr: "3.3.3.3",
			},
		})

		clusterState.UpdateLiveness(20.0)

		node, _ := clusterState.Node("node-2")
		assert.False(t, node.Unreachable)
		node, _ = clusterState.Node("node-3")
		assert.True(t, node.Unreachable)

		suspicionLevels["node-3"] = 5.0

		clusterState.UpdateLiveness(20.0)

		node, _ = clusterState.Node("node-3")
		assert.False(t, node.Unreachable)
	})

	t.Run("watch", func(t *testing.T) {
		suspicionLevels := map[string]float64{
			"node-2": 25.0,
		}
		watcher := &fakeWatcher{}
		clusterState := newClusterState(
			"node-1", "1.1.1.1:1", &fakeFailureDetector{suspicionLevels}, watcher,
		)
		clusterState.ApplyDelta(delta{
			{
				ID:   "node-2",
				Addr: "2.2.2.2",
			},
		})

		clusterState.UpdateLiveness(20.0)

		assert.Equal(t, []string{"node-2"}, watcher.unreachables)

		suspicionLevels["node-2"] = 5.0
		clusterState.UpdateLiveness(20.0)

		assert.Equal(t, []string{"node-2"}, watcher.reachables)
	})
}

type stateUpsert struct {
	NodeID string
	Key    string
	Value  string
}

type stateDelete struct {
	NodeID string
	Key    string
}

type fakeWatcher struct {
	joins        []string
	leaves       []string
	reachables   []string
	unreachables []string
	upserts      []stateUpsert
	deletes      []stateDelete
	expires      []string
}

func (w *fakeWatcher) OnJoin(nodeID string) {
	w.joins = append(w.joins, nodeID)
}

func (w *fakeWatcher) OnLeave(nodeID string) {
	w.leaves = append(w.leaves, nodeID)
}

func (w *fakeWatcher) OnReachable(nodeID string) {
	w.reachables = append(w.reachables, nodeID)
}

func (w *fakeWatcher) OnUnreachable(nodeID string) {
	w.unreachables = append(w.unreachables, nodeID)
}

func (w *fakeWatcher) OnUpsertKey(nodeID, key, value string) {
	w.upserts = append(w.upserts, stateUpsert{
		NodeID: nodeID,
		Key:    key,
		Value:  value,
	})
}

func (w *fakeWatcher) OnDeleteKey(nodeID, key string) {
	w.deletes = append(w.deletes, stateDelete{
		NodeID: nodeID,
		Key:    key,
	})
}

func (w *fakeWatcher) OnExpired(nodeID string) {
	w.expires = append(w.expires, nodeID)
}

var _ Watcher = &fakeWatcher{}

type fakeFailureDetector struct {
	suspicionLevels map[string]float64
}

func (d *fakeFailureDetector) Report(_ string) {
}

func (d *fakeFailureDetector) SuspicionLevel(nodeID string) float64 {
	return d.suspicionLevels[nodeID]
}

func (d *fakeFailureDetector) Remove(_ string) {
}
