package cluster

import (
	"sort"
	"testing"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/stretchr/testify/assert"
)

func TestState_LocalNode(t *testing.T) {
	localNode := &Node{
		ID:     "local",
		Status: NodeStatusActive,
	}
	s := NewState(localNode.Copy(), log.NewNopLogger())

	assert.Equal(t, "local", s.LocalID())

	n, ok := s.Node("local")
	assert.True(t, ok)
	assert.Equal(t, localNode, n)

	assert.Equal(t, localNode, s.LocalNode())

	assert.Equal(t, []*Node{localNode}, s.Nodes())
}

func TestState_UpdateLocalEndpoint(t *testing.T) {
	localNode := &Node{
		ID:     "local",
		Status: NodeStatusActive,
	}
	s := NewState(localNode.Copy(), log.NewNopLogger())

	var notifyEndpointID string
	var notifyListeners int
	s.OnLocalEndpointUpdate(func(endpointID string) {
		notifyEndpointID = endpointID
		notifyListeners = s.LocalEndpointListeners(endpointID)
	})

	s.AddLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	n, _ := s.Node("local")
	assert.Equal(t, 1, n.Endpoints["my-endpoint"])

	s.AddLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	n, _ = s.Node("local")
	assert.Equal(t, 2, n.Endpoints["my-endpoint"])

	s.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	n, _ = s.Node("local")
	assert.Equal(t, 1, n.Endpoints["my-endpoint"])

	s.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 0, notifyListeners)
	n, _ = s.Node("local")
	assert.Equal(t, 0, n.Endpoints["my-endpoint"])

	// Removing an endpoint when none exist should have no affect.
	s.RemoveLocalEndpoint("my-endpoint")
	assert.Equal(t, "my-endpoint", notifyEndpointID)
	assert.Equal(t, 0, notifyListeners)
	n, _ = s.Node("local")
	assert.Equal(t, 0, n.Endpoints["my-endpoint"])
}

func TestState_AddNode(t *testing.T) {
	t.Run("add node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		n, ok := s.Node("remote")
		assert.True(t, ok)
		assert.Equal(t, newNode, n)

		nodes := s.Nodes()
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
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)

		// Attempting to add a node with the same ID should succeed.
		newNode = &Node{
			ID:     "remote",
			Status: NodeStatusUnreachable,
		}
		s.AddNode(newNode)

		n, ok := s.Node("remote")
		assert.True(t, ok)
		assert.Equal(t, newNode, n)
	})

	t.Run("add local node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		// Add a new node with the same ID as the local node. The local node
		// should not be updated.
		newNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		assert.Equal(t, localNode, s.LocalNode())
	})
}

func TestState_RemoveNode(t *testing.T) {
	t.Run("remove node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusUnreachable,
		}
		s.AddNode(newNode)
		assert.True(t, s.RemoveNode(newNode.ID))
		_, ok := s.Node("remote")
		assert.False(t, ok)

		assert.Equal(t, []*Node{localNode}, s.Nodes())
	})

	t.Run("remove local node", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		// Attempting to delete the local node should have no affect.
		assert.False(t, s.RemoveNode(localNode.ID))
		assert.Equal(t, localNode, s.LocalNode())
	})
}

func TestState_UpdateRemoteStatus(t *testing.T) {
	t.Run("update status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteStatus("remote", NodeStatusUnreachable))

		n, _ := s.Node("remote")
		assert.Equal(t, NodeStatusUnreachable, n.Status)
	})

	t.Run("update local status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, s.UpdateRemoteStatus("local", NodeStatusUnreachable))
		assert.Equal(t, localNode, s.LocalNode())
	})
}

func TestState_UpdateRemoteEndpoint(t *testing.T) {
	t.Run("update endpoint", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusUnreachable,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint", 7))

		n, _ := s.Node("remote")
		assert.Equal(t, 7, n.Endpoints["my-endpoint"])
	})

	t.Run("update local status", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, s.UpdateRemoteEndpoint("local", "my-endpoint", 7))
		assert.Equal(t, localNode, s.LocalNode())
	})
}

func TestState_RemoveRemoteEndpoint(t *testing.T) {
	t.Run("update endpoint", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint", 7))
		assert.True(t, s.RemoveRemoteEndpoint("remote", "my-endpoint"))

		n, _ := s.Node("remote")
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
		s := NewState(localNode.Copy(), log.NewNopLogger())

		// Attempting to update the local node should have no affect.
		assert.False(t, s.RemoveRemoteEndpoint("local", "my-endpoint"))
		assert.Equal(t, localNode, s.LocalNode())
	})
}

func TestState_LookupEndpoint(t *testing.T) {
	t.Run("found", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint-1", 7))

		node, ok := s.LookupEndpoint("my-endpoint-1")
		assert.True(t, ok)
		assert.Equal(t, newNode, node)
	})

	t.Run("ignore unreachable", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusUnreachable,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint-1", 7))

		_, ok := s.LookupEndpoint("my-endpoint-1")
		assert.False(t, ok)
	})

	t.Run("ignore left", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusLeft,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint-1", 7))

		_, ok := s.LookupEndpoint("my-endpoint-1")
		assert.False(t, ok)
	})

	t.Run("not found", func(t *testing.T) {
		localNode := &Node{
			ID:     "local",
			Status: NodeStatusActive,
		}
		s := NewState(localNode.Copy(), log.NewNopLogger())

		newNode := &Node{
			ID:     "remote",
			Status: NodeStatusActive,
		}
		s.AddNode(newNode)
		assert.True(t, s.UpdateRemoteEndpoint("remote", "my-endpoint-1", 7))

		_, ok := s.LookupEndpoint("my-endpoint-2")
		assert.False(t, ok)
	})
}
