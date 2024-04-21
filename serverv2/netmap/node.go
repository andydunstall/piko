package netmap

import (
	"crypto/rand"
	"math/big"
)

var (
	alphaNumericChars = []byte("abcdefghijklmnopqrstuvwxyz1234567890")
)

type NodeStatus string

const (
	NodeStatusJoining NodeStatus = "joining"
	NodeStatusActive  NodeStatus = "active"
)

// Node represents the known state about a node in the cluster.
type Node struct {
	// ID is a unique identifier for the node in the cluster.
	ID string `json:"id"`

	Status NodeStatus `json:"status"`

	// ProxyAddr is the advertised proxy address.
	ProxyAddr string `json:"proxy_addr"`
	// AdminAddr is the advertised admin address.
	AdminAddr string `json:"admin_addr"`
	// GossipAddr is the advertised gossip address.
	GossipAddr string `json:"gossip_addr"`

	// Endpoints contains the active endpoints on the node (endpoints with at
	// least one upstream listener). This maps the active endpoint ID to the
	// number of listeners for that endpoint.
	Endpoints map[string]int `json:"endpoints"`
}

func (n *Node) AddEndpoint(endpointID string) int {
	if n.Endpoints == nil {
		n.Endpoints = make(map[string]int)
	}

	n.Endpoints[endpointID] = n.Endpoints[endpointID] + 1
	return n.Endpoints[endpointID]
}

func (n *Node) RemoveEndpoint(endpointID string) int {
	if n.Endpoints == nil {
		return 0
	}

	numListeners, ok := n.Endpoints[endpointID]
	if !ok {
		return 0
	}
	if numListeners > 1 {
		n.Endpoints[endpointID] = numListeners - 1
		return n.Endpoints[endpointID]
	}
	delete(n.Endpoints, endpointID)
	return 0
}

func (n *Node) Copy() *Node {
	endpoints := make(map[string]int)
	for endpointID, numListeners := range n.Endpoints {
		endpoints[endpointID] = numListeners
	}
	return &Node{
		ID:         n.ID,
		Status:     n.Status,
		ProxyAddr:  n.ProxyAddr,
		AdminAddr:  n.AdminAddr,
		GossipAddr: n.GossipAddr,
		Endpoints:  endpoints,
	}
}

func GenerateNodeID() string {
	b := make([]byte, 10)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphaNumericChars))))
		if err != nil {
			// We don't expect to ever get an error so panic rather than try to
			// handle.
			panic("failed to generate random number: " + err.Error())
		}
		b[i] = alphaNumericChars[n.Int64()]
	}
	return string(b)
}
