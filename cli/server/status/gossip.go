package status

import (
	"fmt"
	"os"
	"sort"

	"github.com/andydunstall/piko/pkg/gossip"
	"github.com/andydunstall/piko/server/status/client"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newGossipCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gossip",
		Short: "inspect gossip state",
	}

	cmd.AddCommand(newGossipNodesCommand(c))
	cmd.AddCommand(newGossipNodeCommand(c))

	return cmd
}

func newGossipNodesCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "inspect gossip nodes",
		Long: `Inspect gossip nodes.

Queries the server for the metadata for each known gossip node in the
cluster.

Examples:
  piko server status gossip nodes
`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		showGossipNodes(c)
	}

	return cmd
}

type gossipNodesOutput struct {
	Nodes []gossip.NodeMetadata `json:"nodes"`
}

func showGossipNodes(c *client.Client) {
	gossip := client.NewGossip(c)

	nodes, err := gossip.Nodes()
	if err != nil {
		fmt.Printf("failed to get gossip nodes: %s\n", err.Error())
		os.Exit(1)
	}

	// Sort by ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	output := gossipNodesOutput{
		Nodes: nodes,
	}
	b, _ := yaml.Marshal(output)
	fmt.Println(string(b))
}

func newGossipNodeCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Args:  cobra.ExactArgs(1),
		Short: "inspect a gossip node",
		Long: `Inspect a gossip node.

Queries the server for the known state of the gossip node with the given ID.

Examples:
  piko server status gossip node bbc69214
`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		showGossipNode(args[0], c)
	}

	return cmd
}

func showGossipNode(nodeID string, c *client.Client) {
	gossip := client.NewGossip(c)

	node, err := gossip.Node(nodeID)
	if err != nil {
		fmt.Printf("failed to get gossip node: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(node)
	fmt.Println(string(b))
}
