package gossip

type NodeMetadata struct {
	ID string `json:"id"`
}

type NodeState struct {
	NodeMetadata
}
