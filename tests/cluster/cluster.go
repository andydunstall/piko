package cluster

import "fmt"

type Cluster struct {
	nodes []*Node
}

func NewCluster(opts ...Option) (*Cluster, error) {
	options := options{
		nodes: 3,
	}
	for _, o := range opts {
		o.apply(&options)
	}

	var nodes []*Node
	for i := 0; i != options.nodes; i++ {
		node, err := NewNode(
			fmt.Sprintf("node-%d", i),
			opts...,
		)
		if err != nil {
			return nil, fmt.Errorf("node: %w", err)
		}
		nodes = append(nodes, node)
	}

	return &Cluster{
		nodes: nodes,
	}, nil
}

func (c *Cluster) Start() error {
	for _, node := range c.nodes {
		node.Start()
	}

	var nodeAddrs []string
	for _, node := range c.nodes {
		nodeAddrs = append(nodeAddrs, node.GossipAddr())
	}
	for _, node := range c.nodes {
		if _, err := node.gossiper.JoinOnBoot(nodeAddrs); err != nil {
			return fmt.Errorf("join: %w", err)
		}
	}

	return nil
}

func (c *Cluster) Stop() {
	for _, node := range c.nodes {
		node.Stop()
	}
}

func (c *Cluster) Nodes() []*Node {
	return c.nodes
}
