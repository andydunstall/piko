package client

import (
	"encoding/json"
	"fmt"

	"github.com/dragonflydb/piko/server/cluster"
)

type Cluster struct {
	client *Client
}

func NewCluster(client *Client) *Cluster {
	return &Cluster{
		client: client,
	}
}

func (c *Cluster) Nodes() ([]*cluster.NodeMetadata, error) {
	r, err := c.client.Request("/status/cluster/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var nodes []*cluster.NodeMetadata
	if err := json.NewDecoder(r).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return nodes, nil
}

func (c *Cluster) Node(nodeID string) (*cluster.Node, error) {
	r, err := c.client.Request("/status/cluster/nodes/" + nodeID)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var node cluster.Node
	if err := json.NewDecoder(r).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &node, nil
}
