package status

import (
	"fmt"
	"os"
	"sort"

	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	clusterServer "github.com/andydunstall/piko/server/cluster"
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
	Nodes []*clusterServer.NodeMetadata `json:"nodes"`
}

func showClusterNodes(c *client.Client) {
	cluster := client.NewCluster(c)

	nodes, err := cluster.Nodes()
	if err != nil {
		fmt.Printf("failed to get cluster nodes: %s\n", err.Error())
		os.Exit(1)
	}

	// Convert nodes to NodeMetadata.
	metadata := make([]*clusterServer.NodeMetadata, len(nodes))
	for i, node := range nodes {
		metadata[i] = &clusterServer.NodeMetadata{
			ID:        node.ID,
			Status:    node.Status,
			ProxyAddr: node.ProxyAddr,
			AdminAddr: node.AdminAddr,
			Endpoints: node.Endpoints,
			Upstreams: node.Upstreams,
		}
	}

	// Sort by ID.
	sort.Slice(metadata, func(i, j int) bool {
		return metadata[i].ID < metadata[j].ID
	})

	// Calculate total and average connections.
	average, total := CalculateClusterMetrics(metadata)

	output := struct {
		Nodes              []*clusterServer.NodeMetadata `json:"nodes"`
		TotalConnections   int                           `json:"total_connections"`
		AverageConnections float64                       `json:"average_connections"`
	}{
		Nodes:              metadata,
		TotalConnections:   total,
		AverageConnections: average,
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

	// Fetch the node by ID
	node, err := cluster.Node(nodeID)
	if err != nil {
		fmt.Printf("failed to get cluster node: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	fmt.Printf("Node ID: %s\n", node.ID)
	fmt.Printf("Total Connections: %d\n", node)

	b, _ := yaml.Marshal(node)
	fmt.Print(string(b))
}

func CalculateClusterMetrics(nodes []*clusterServer.NodeMetadata) (average float64, total int) {
	for _, node := range nodes {
		total += node.Upstreams
	}
	if len(nodes) > 0 {
		average = float64(total) / float64(len(nodes))
	}
	return average, total
}
