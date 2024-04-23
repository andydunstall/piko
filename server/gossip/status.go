package gossip

import (
	"net/http"
	"sort"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/server/status"
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
	group.GET("/members", s.listMembersRoute)
	group.GET("/members/:id", s.getMemberRoute)
}

func (s *Status) listMembersRoute(c *gin.Context) {
	members := s.gossip.MembersMetadata(kite.MemberFilter{
		Local: true,
	})
	c.JSON(http.StatusOK, members)
}

func (s *Status) getMemberRoute(c *gin.Context) {
	id := c.Param("id")
	state, ok := s.gossip.MemberState(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}

	// Sort state by version.
	sort.Slice(state.State, func(i, j int) bool {
		return state.State[i].Version < state.State[j].Version
	})

	c.JSON(http.StatusOK, state)
}

var _ status.Handler = &Status{}
