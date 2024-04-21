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
	NodeStatusLeaving NodeStatus = "leaving"
)

// Node represents the known state about a node in the cluster.
type Node struct {
	// ID is a unique identifier for the node in the cluster.
	ID string `json:"id"`

	Status NodeStatus `json:"status"`

	// GossipAddr is the advertised gossip address.
	GossipAddr string `json:"gossip_addr"`
}

func (n *Node) Copy() *Node {
	return &Node{
		ID:         n.ID,
		Status:     n.Status,
		GossipAddr: n.GossipAddr,
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
