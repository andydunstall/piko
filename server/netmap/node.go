package netmap

// Node represents the known state about a node in the cluster.
type Node struct {
	// ID is a unique identifier for the node in the cluster.
	ID string

	// HTTPAddr is the advertised HTTP address
	HTTPAddr string

	// GossipAddr is the advertised gossip address.
	GossipAddr string

	// Endpoints contains the active endpoints on the node. This maps the
	// active endpoint ID to the number of listeners for that endpoint.
	Endpoints map[string]int
}
