package netmap

// NetworkMap represents the known state of the cluster as seen by the local
// node.
//
// This map is eventually consistent. The state is propagated among the nodes
// in the cluster using gossip.
type NetworkMap struct {
}

func NewNetworkMap() *NetworkMap {
	return &NetworkMap{}
}
