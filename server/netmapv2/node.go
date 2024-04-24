package netmap

// NodeStatus contains the known status as set by the node itself.
type NodeStatus string

const (
	// NodeStatusJoining means the node is joining the cluster though is not
	// yet accepting traffic.
	NodeStatusJoining NodeStatus = "joining"
	// NodeStatusActive means the node is healthy and accepting traffic.
	NodeStatusActive NodeStatus = "active"
	// NodeStatusLeaving means the node is leaving the cluster and no longer
	// accepting traffic.
	NodeStatusLeaving NodeStatus = "leaving"
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

	// Status contains the node status as set by the node itself.
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
