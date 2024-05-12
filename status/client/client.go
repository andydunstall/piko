package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	fspath "path"
	"time"

	"github.com/andydunstall/pico/pkg/gossip"
	"github.com/andydunstall/pico/server/cluster"
)

type Client struct {
	httpClient *http.Client

	url *url.URL

	forward string
}

func NewClient(url *url.URL, forward string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
		url:     url,
		forward: forward,
	}
}

func (c *Client) ProxyEndpoints() (map[string][]string, error) {
	r, err := c.request("/status/proxy/endpoints")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var endpoints map[string][]string
	if err := json.NewDecoder(r).Decode(&endpoints); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return endpoints, nil
}

func (c *Client) ClusterNodes() ([]*cluster.Node, error) {
	r, err := c.request("/status/cluster/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var nodes []*cluster.Node
	if err := json.NewDecoder(r).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return nodes, nil
}

func (c *Client) ClusterNode(nodeID string) (*cluster.Node, error) {
	r, err := c.request("/status/cluster/nodes/" + nodeID)
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

func (c *Client) GossipNodes() ([]gossip.NodeMetadata, error) {
	r, err := c.request("/status/gossip/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var members []gossip.NodeMetadata
	if err := json.NewDecoder(r).Decode(&members); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return members, nil
}

func (c *Client) GossipNode(memberID string) (*gossip.NodeState, error) {
	r, err := c.request("/status/gossip/nodes/" + memberID)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var member gossip.NodeState
	if err := json.NewDecoder(r).Decode(&member); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &member, nil
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) request(path string) (io.ReadCloser, error) {
	url := new(url.URL)
	*url = *c.url

	if c.forward != "" {
		url.RawQuery = "forward=" + c.forward
	}

	url.Path = fspath.Join(url.Path, path)

	req, err := http.NewRequest(http.MethodGet, url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()

		return nil, fmt.Errorf("request: bad status: %d", resp.StatusCode)
	}

	return resp.Body, nil
}
