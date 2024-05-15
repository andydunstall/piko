package gossip

import (
	"net/http"
	"sort"

	"github.com/andydunstall/piko/server/status"
	"github.com/gin-gonic/gin"
)

type Status struct {
	gossip *Gossip
}

func NewStatus(gossip *Gossip) *Status {
	return &Status{
		gossip: gossip,
	}
}

func (s *Status) Register(group *gin.RouterGroup) {
	group.GET("/nodes", s.listNodesRoute)
	group.GET("/nodes/:id", s.getNodeRoute)
}

func (s *Status) listNodesRoute(c *gin.Context) {
	nodes := s.gossip.Nodes()

	// Sort by node ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	c.JSON(http.StatusOK, nodes)
}

func (s *Status) getNodeRoute(c *gin.Context) {
	id := c.Param("id")
	state, ok := s.gossip.NodeState(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, state)
}

var _ status.Handler = &Status{}
