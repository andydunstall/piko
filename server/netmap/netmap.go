package netmap

// NetworkMap represents the known state of the cluster as seen by the local
// node.
//
// This map is eventually consistent. The state is propagated among the nodes
// in the cluster using gossip.
type NetworkMap struct {
}

func NewNetworkMap(_ *Node) *NetworkMap {
	return &NetworkMap{}
}

func (m *NetworkMap) LocalNode() *Node {
	return nil
}

func (m *NetworkMap) NodeByID(_ string) (*Node, bool) {
	return nil, false
}

func (m *NetworkMap) UpdateNodeByID(_ string, _ func(n *Node)) bool {
	return false
}

func (m *NetworkMap) DeleteNodeByID(_ string) bool {
	return false
}

func (m *NetworkMap) AddNode(_ *Node) {
}

func (m *NetworkMap) OnLocalStatusUpdated(_ func(status NodeStatus)) {
}

func (m *NetworkMap) OnLocalEndpointUpdated(_ func(endpointID string, numListeners int)) {
}

func (m *NetworkMap) OnLocalEndpointRemoved(_ func(endpointID string)) {
}
