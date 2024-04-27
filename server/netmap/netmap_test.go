package netmap

import (
	"sort"
	"testing"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestNetworkMap_LocalNode(t *testing.T) {
	localNode := &Node{
		ID:     "local",
		Status: NodeStatusActive,
	}
	m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

	assert.Equal(t, "local", m.LocalID())

	n, ok := m.Node("local")
	assert.True(t, ok)
	assert.Equal(t, localNode, n)

	assert.Equal(t, localNode, m.LocalNode())

	assert.Equal(t, []*Node{localNode}, m.Nodes())
}

func TestNetworkMap_UpdateLocalEndpoint(t *testing.T) {
	localNode := &Node{
		ID:     "local",
		Status: NodeStatusActive,
	}
	m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

	var notifyEndpointID string
	var notifyListeners int
	m.OnLocalEndpointUpdate(func(endpointID string, listeners int) {
		notifyEndpointID = endpointID
		notifyListeners = listeners
	})

	m.AddLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 1, notifyListeners)
	n, _ := m.Node("local")
	assert.Equal(t, 1, n.Endpoints["my-endpoint"])

	m.AddLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 2, notifyListeners)
	n, _ = m.Node("local")
	assert.Equal(t, 2, n.Endpoints["my-endpoint"])

	m.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 1, notifyListeners)
	n, _ = m.Node("local")
	assert.Equal(t, 1, n.Endpoints["my-endpoint"])

	m.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 0, notifyListeners)
	n, _ = m.Node("local")
	assert.Equal(t, 0, n.Endpoints["my-endpoint"])

	// Removing an endpoint when none exist should have no affect.
	m.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 0, notifyListeners)
	n, _ = m.Node("local")
	assert.Equal(t, 0, n.Endpoints["my-endpoint"])
}

func TestNetworkMap_AddNode(t *testing.T) {
	t.Run("add node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		m.AddNode(newNode)
		n, ok := m.Node("remote")
		assert.True(t, ok)
		assert.Equal(t, newNode, n)

		nodes := m.Nodes()
		// Sort to simplify comparison.
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].ID < nodes[j].ID
		})
		assert.Equal(t, []*Node{localNode, newNode}, nodes)
	})

	t.Run("add node already exists", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		m.AddNode(newNode)

		// Attempting to add a node with the same ID should succeed.
		newNode = &Node{
			ID:     "remote",
			Status: NodeStatusDown,
		}
		m.AddNode(newNode)

		n, ok := m.Node("remote")
		assert.True(t, ok)
		assert.Equal(t, newNode, n)
	})

	t.Run("add local node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		// Add a new node with the same ID as the local node. The local node
		// should not be updated.
		newNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m.AddNode(newNode)
		assert.Equal(t, localNode, m.LocalNode())
	})
}

func TestNetworkMap_RemoveNode(t *testing.T) {
	t.Run("remove node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusDown,
		}
		m.AddNode(newNode)
		assert.True(t, m.RemoveNode(newNode.ID))
		_, ok := m.Node("remote")
		assert.False(t, ok)

		assert.Equal(t, []*Node{localNode}, m.Nodes())
	})

	t.Run("remove local node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		// Attempting to delete the local node should have no affect.
		assert.False(t, m.RemoveNode(localNode.ID))
		assert.Equal(t, localNode, m.LocalNode())
	})
}

func TestNetworkMap_UpdateRemoteStatus(t *testing.T) {
	t.Run("update status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		m.AddNode(newNode)
		assert.True(t, m.UpdateRemoteStatus("remote", NodeStatusDown))

		n, _ := m.Node("remote")
		assert.Equal(t, NodeStatusDown, n.Status)
	})

	t.Run("update local status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, m.UpdateRemoteStatus("local", NodeStatusDown))
		assert.Equal(t, localNode, m.LocalNode())
	})
}

func TestNetworkMap_UpdateRemoteEndpoint(t *testing.T) {
	t.Run("update endpoint", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusDown,
		}
		m.AddNode(newNode)
		assert.True(t, m.UpdateRemoteEndpoint("remote", "my-endpoint", 7))

		n, _ := m.Node("remote")
		assert.Equal(t, 7, n.Endpoints["my-endpoint"])
	})

	t.Run("update local status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, m.UpdateRemoteEndpoint("local", "my-endpoint", 7))
		assert.Equal(t, localNode, m.LocalNode())
	})
}

func TestNetworkMap_RemoveRemoteEndpoint(t *testing.T) {
	t.Run("update endpoint", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		m.AddNode(newNode)
		assert.True(t, m.UpdateRemoteEndpoint("remote", "my-endpoint", 7))
		assert.True(t, m.RemoveRemoteEndpoint("remote", "my-endpoint"))

		n, _ := m.Node("remote")
		assert.Equal(t, 0, n.Endpoints["my-endpoint"])
	})

	t.Run("update local status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
			Endpoints: map[string]int{
				"my-endpoint": 7,
			},
		}
		m := NewNetworkMap(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, m.RemoveRemoteEndpoint("local", "my-endpoint"))
		assert.Equal(t, localNode, m.LocalNode())
	})
}
