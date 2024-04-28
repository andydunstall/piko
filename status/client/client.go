package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	fspath "path"
	"time"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/server/netmap"
)

type Client struct {
	httpClient *http.Client

	url *url.URL
}

func NewClient(url *url.URL) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Second * 15,
		},
		url: url,
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

func (c *Client) NetmapNodes() ([]*netmap.Node, error) {
	r, err := c.request("/status/netmap/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var nodes []*netmap.Node
	if err := json.NewDecoder(r).Decode(&nodes); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return nodes, nil
}

func (c *Client) NetmapNode(nodeID string) (*netmap.Node, error) {
	r, err := c.request("/status/netmap/nodes/" + nodeID)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var node netmap.Node
	if err := json.NewDecoder(r).Decode(&node); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &node, nil
}

func (c *Client) GossipNodes() ([]kite.NodeMetadata, error) {
	r, err := c.request("/status/gossip/nodes")
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var members []kite.NodeMetadata
	if err := json.NewDecoder(r).Decode(&members); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return members, nil
}

func (c *Client) GossipNode(memberID string) (*kite.NodeState, error) {
	r, err := c.request("/status/gossip/nodes/" + memberID)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var member kite.NodeState
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
