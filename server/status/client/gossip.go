package client

import (
	"encoding/json"
	"fmt"

	"github.com/dragonflydb/piko/pkg/gossip"
)

type Gossip struct {
	client *Client
}

func NewGossip(client *Client) *Gossip {
	return &Gossip{
		client: client,
	}
}

func (c *Gossip) Nodes() ([]gossip.NodeMetadata, error) {
	r, err := c.client.Request("/status/gossip/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var nodes []gossip.NodeMetadata
	if err := json.NewDecoder(r).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return nodes, nil
}

func (c *Gossip) Node(nodeID string) (*gossip.NodeState, error) {
	r, err := c.client.Request("/status/gossip/nodes/" + nodeID)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var node gossip.NodeState
	if err := json.NewDecoder(r).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &node, nil
}
