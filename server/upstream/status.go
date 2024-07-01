package upstream

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/andydunstall/piko/server/status"
)

type Status struct {
	manager *LoadBalancedManager
}

func NewStatus(manager *LoadBalancedManager) *Status {
	return &Status{
		manager: manager,
	}
}

func (s *Status) Register(group *gin.RouterGroup) {
	group.GET("/endpoints", s.listEndpointsRoute)
}

func (s *Status) listEndpointsRoute(c *gin.Context) {
	endpoints := s.manager.Endpoints()
	c.JSON(http.StatusOK, endpoints)
}

var _ status.Handler = &Status{}
