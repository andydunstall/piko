package netmap

import (
	"crypto/rand"
	"math/big"
)

var (
	alphaNumericChars = []byte("abcdefghijklmnopqrstuvwxyz1234567890")
)

// NodeStatus contains the known status of a node.
type NodeStatus string

const (
	// NodeStatusActive means the node is healthy and accepting traffic.
	NodeStatusActive NodeStatus = "active"
	// NodeStatusDown means the node is considered down.
	NodeStatusDown NodeStatus = "down"
	// NodeStatusLeft means the node has left the cluster.
	NodeStatusLeft NodeStatus = "left"
)

// Node represents the known state about a node in the cluster.
//
// Note to ensure updates are propagated, never update a node directly, only
// ever update via the NetworkMap.
type Node struct {
	// ID is a unique identifier for the node in the cluster.
	//
	// The ID is immutable.
	ID string `json:"id"`

	// Status contains the known status of the node.
	Status NodeStatus `json:"status"`

	// ProxyAddr is the advertised proxy address.
	//
	// The address is immutable.
	ProxyAddr string `json:"proxy_addr"`

	// AdminAddr is the advertised admin address.
	//
	// The address is immutable.
	AdminAddr string `json:"admin_addr"`

	// Endpoints contains the known active endpoints on the node (endpoints
	// with at least one upstream listener).
	//
	// This maps the endpoint ID to the number of known listeners for that
	// endpoint.
	Endpoints map[string]int `json:"endpoints"`
}

func (n *Node) Copy() *Node {
	var endpoints map[string]int
	if len(n.Endpoints) > 0 {
		endpoints = make(map[string]int)
		for endpointID, listeners := range n.Endpoints {
			endpoints[endpointID] = listeners
		}
	}
	return &Node{
		ID:        n.ID,
		Status:    n.Status,
		ProxyAddr: n.ProxyAddr,
		AdminAddr: n.AdminAddr,
		Endpoints: endpoints,
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
