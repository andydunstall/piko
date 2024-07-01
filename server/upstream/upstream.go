package upstream

import (
	"net"

	"github.com/hashicorp/yamux"

	"github.com/andydunstall/piko/server/cluster"
)

// Upstream represents an upstream for a given endpoint.
//
// An upstream may be an upstream service connected to the local node, or
// another Piko server node.
type Upstream interface {
	EndpointID() string
	Dial() (net.Conn, error)
	// Forward indicates whether the upstream is forwarding traffic to a remote
	// node rather than a client listener.
	Forward() bool
}

// ConnUpstream represents a connection to an upstream service thats connected
// to the local node.
type ConnUpstream struct {
	endpointID string
	sess       *yamux.Session
}

func NewConnUpstream(endpointID string, sess *yamux.Session) *ConnUpstream {
	return &ConnUpstream{
		endpointID: endpointID,
		sess:       sess,
	}
}

func (u *ConnUpstream) EndpointID() string {
	return u.endpointID
}

func (u *ConnUpstream) Dial() (net.Conn, error) {
	return u.sess.OpenStream()
}

func (u *ConnUpstream) Forward() bool {
	return false
}

// NodeUpstream represents a remote Piko server node.
type NodeUpstream struct {
	endpointID string
	node       *cluster.Node
}

func NewNodeUpstream(endpointID string, node *cluster.Node) *NodeUpstream {
	return &NodeUpstream{
		endpointID: endpointID,
		node:       node,
	}
}

func (u *NodeUpstream) EndpointID() string {
	return u.endpointID
}

func (u *NodeUpstream) Dial() (net.Conn, error) {
	return net.Dial("tcp", u.node.ProxyAddr)
}

func (u *NodeUpstream) Forward() bool {
	return true
}
