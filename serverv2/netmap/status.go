package netmap

import (
	"net/http"

	"github.com/andydunstall/pico/serverv2/status"
	"github.com/gin-gonic/gin"
)

type Status struct {
	networkMap *NetworkMap
}

func NewStatus(networkMap *NetworkMap) *Status {
	return &Status{
		networkMap: networkMap,
	}
}

func (s *Status) Register(group *gin.RouterGroup) {
	group.GET("/nodes", s.listNodesRoute)
	group.GET("/nodes/local", s.getLocalNodeRoute)
	group.GET("/nodes/:id", s.getNodeRoute)
}

func (s *Status) listNodesRoute(c *gin.Context) {
	nodes := s.networkMap.Nodes()
	c.JSON(http.StatusOK, nodes)
}

func (s *Status) getLocalNodeRoute(c *gin.Context) {
	node := s.networkMap.LocalNode()
	c.JSON(http.StatusOK, node)
}

func (s *Status) getNodeRoute(c *gin.Context) {
	id := c.Param("id")
	node, ok := s.networkMap.Node(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	c.JSON(http.StatusOK, node)
}

var _ status.Handler = &Status{}
