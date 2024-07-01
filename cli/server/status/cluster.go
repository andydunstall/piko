package status

import (
	"fmt"
	"os"
	"sort"

	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/andydunstall/piko/server/cluster"
	"github.com/andydunstall/piko/server/status/client"
)

func newClusterCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "inspect proxy cluster",
	}

	cmd.AddCommand(newClusterNodesCommand(c))
	cmd.AddCommand(newClusterNodeCommand(c))

	return cmd
}

func newClusterNodesCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "inspect cluster nodes",
		Long: `Inspect cluster nodes.

Queries the server for the set of nodes the cluster that this node knows about.
The output contains the state of each known node.

Examples:
  piko server status cluster nodes
`,
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		showClusterNodes(c)
	}

	return cmd
}

type clusterNodesOutput struct {
	Nodes []*cluster.NodeMetadata `json:"nodes"`
}

func showClusterNodes(c *client.Client) {
	cluster := client.NewCluster(c)

	nodes, err := cluster.Nodes()
	if err != nil {
		fmt.Printf("failed to get cluster nodes: %s\n", err.Error())
		os.Exit(1)
	}

	// Sort by ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	output := clusterNodesOutput{
		Nodes: nodes,
	}
	b, _ := yaml.Marshal(output)
	fmt.Print(string(b))
}

func newClusterNodeCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Args:  cobra.ExactArgs(1),
		Short: "inspect cluster node",
		Long: `Inspect a cluster node.

Queries the server for the known state of the node with the given ID. Or use
a node ID of 'local' to query the local node.

Examples:
  # Inspect node bbc69214.
  piko server status cluster node bbc69214

  # Inspect local node.
  piko server status cluster node local
`,
	}

	cmd.Run = func(_ *cobra.Command, args []string) {
		showClusterNode(args[0], c)
	}

	return cmd
}

func showClusterNode(nodeID string, c *client.Client) {
	cluster := client.NewCluster(c)

	node, err := cluster.Node(nodeID)
	if err != nil {
		fmt.Printf("failed to get cluster nodes: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(node)
	fmt.Print(string(b))
}
