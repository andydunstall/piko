package cluster

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/dragonflydb/piko/server/status"
)

type Status struct {
	state *State
}

func NewStatus(state *State) *Status {
	return &Status{
		state: state,
	}
}

func (s *Status) Register(group *gin.RouterGroup) {
	group.GET("/nodes", s.listNodesRoute)
	group.GET("/nodes/local", s.getLocalNodeRoute)
	group.GET("/nodes/:id", s.getNodeRoute)
}

func (s *Status) listNodesRoute(c *gin.Context) {
	nodes := s.state.NodesMetadata()
	c.JSON(http.StatusOK, nodes)
}

func (s *Status) getLocalNodeRoute(c *gin.Context) {
	node := s.state.LocalNode()
	c.JSON(http.StatusOK, node)
}

func (s *Status) getNodeRoute(c *gin.Context) {
	id := c.Param("id")
	node, ok := s.state.Node(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, node)
}

var _ status.Handler = &Status{}
