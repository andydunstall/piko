package cluster

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/andydunstall/piko/bench/config"
)

var (
	alphaNumericChars = []byte("abcdefghijklmnopqrstuvwxyz1234567890")
)

// NodeStatus contains the known status of a node.
type NodeStatus string

const (
	// NodeStatusActive means the node is healthy and accepting traffic.
	NodeStatusActive NodeStatus = "active"
	// NodeStatusUnreachable means the node is considered unreachable.
	NodeStatusUnreachable NodeStatus = "unreachable"
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

func (n *Node) Metadata() *NodeMetadata {
	upstreams := 0
	for _, endpointUpstreams := range n.Endpoints {
		upstreams += endpointUpstreams
	}
	return &NodeMetadata{
		ID:        n.ID,
		Status:    n.Status,
		ProxyAddr: n.ProxyAddr,
		AdminAddr: n.AdminAddr,
		Endpoints: len(n.Endpoints),
		Upstreams: upstreams,
	}
}

func (n *Node) TotalConnections(nodes []*NodeMetadata) int {
	total := 0
	for _, node := range nodes {
		total += node.Upstreams
	}
	return total
}

func (n *Node) AverageConnections(metadata []*NodeMetadata) float64 {
	if len(metadata) == 0 {
		return 0
	}
	return float64(n.TotalConnections(metadata)) / float64(len(metadata))
}

func (n *Node) maybeShedConnections(nodes []*NodeMetadata, clusterAverage float64, threshold float64, shedRate float64, totalConnections int) {
	if totalConnections < 10 {
		return
	}

	excess := float64(n.TotalConnections(nodes)) - (clusterAverage * (1.0 + threshold))
	if excess > 0 {
		shedCount := int(excess * shedRate)
		n.shedConnections(shedCount)
	}
}

// Shed connections by disconnecting a specified number of listeners.
func (n *Node) shedConnections(count int) {
	removed := 0
	for endpoint, listenerCount := range n.Endpoints {
		if removed >= count {
			break
		}

		disconnect := min(count-removed, listenerCount)
		n.Endpoints[endpoint] -= disconnect
		removed += disconnect

		// Propagate these changes through gossip.
		if n.Endpoints[endpoint] <= 0 {
			delete(n.Endpoints, endpoint)
		}
	}
}

// NodeMetadata contains metadata fields from Node.
type NodeMetadata struct {
	ID        string     `json:"id"`
	Status    NodeStatus `json:"status"`
	ProxyAddr string     `json:"proxy_addr"`
	AdminAddr string     `json:"admin_addr"`
	Endpoints int        `json:"endpoints"`
	// Upstreams is the number of upstreams connected to this node.
	Upstreams int `json:"upstreams"`
}

func GenerateNodeID() string {
	b := make([]byte, 7)
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

func (n *Node) StartRebalancing(nodes []*Node, interval time.Duration, config config.Config) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// Dynamically calculate metadata for all nodes.
		var nodeMetadataList []*NodeMetadata
		for _, node := range nodes {
			nodeMetadataList = append(nodeMetadataList, node.Metadata())
		}

		totalConnections := n.TotalConnections(nodeMetadataList)
		clusterAverage := n.AverageConnections(nodeMetadataList)

		fmt.Printf("Rebalancing: Cluster Average Connections: %.2f\n", clusterAverage)

		// Shed connections for nodes exceeding the threshold.
		for _, node := range nodes {
			node.maybeShedConnections(nodeMetadataList, clusterAverage, config.RebalanceThreshold, config.ShedRate, totalConnections)
		}
	}
}
