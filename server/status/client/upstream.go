package client

import (
	"encoding/json"
	"fmt"
)

type Upstream struct {
	client *Client
}

func NewUpstream(client *Client) *Upstream {
	return &Upstream{
		client: client,
	}
}

func (c *Upstream) Endpoints() (map[string]int, error) {
	r, err := c.client.Request("/status/upstream/endpoints")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	endpoints := make(map[string]int)
	if err := json.NewDecoder(r).Decode(&endpoints); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return endpoints, nil
}
