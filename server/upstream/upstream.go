package upstream

import (
	"crypto/tls"
	"net"

	"github.com/andydunstall/yamux"

	"github.com/dragonflydb/piko/server/cluster"
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
	tlsConfig  *tls.Config
}

func NewNodeUpstream(endpointID string, node *cluster.Node, tlsConfig *tls.Config) *NodeUpstream {
	return &NodeUpstream{
		endpointID: endpointID,
		node:       node,
		tlsConfig:  tlsConfig,
	}
}

func (u *NodeUpstream) EndpointID() string {
	return u.endpointID
}

func (u *NodeUpstream) Dial() (net.Conn, error) {
	if u.tlsConfig != nil {
		return tls.Dial("tcp", u.node.ProxyAddr, u.tlsConfig)
	}

	return net.Dial("tcp", u.node.ProxyAddr)
}

func (u *NodeUpstream) Forward() bool {
	return true
}
