package proxy

import (
	"net/http"

	"github.com/andydunstall/piko/server/status"
	"github.com/gin-gonic/gin"
)

type Status struct {
	proxy *Proxy
}

func NewStatus(proxy *Proxy) *Status {
	return &Status{
		proxy: proxy,
	}
}

func (s *Status) Register(group *gin.RouterGroup) {
	group.GET("/endpoints", s.listEndpointsRoute)
}

func (s *Status) listEndpointsRoute(c *gin.Context) {
	endpoints := s.proxy.ConnAddrs()
	c.JSON(http.StatusOK, endpoints)
}

var _ status.Handler = &Status{}
