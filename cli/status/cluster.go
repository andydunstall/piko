package status

import (
	"fmt"
	"net/url"
	"os"
	"sort"

	"github.com/andydunstall/pico/server/cluster"
	"github.com/andydunstall/pico/status/client"
	"github.com/andydunstall/pico/status/config"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newClusterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "inspect proxy cluster",
	}

	cmd.AddCommand(newClusterNodesCommand())
	cmd.AddCommand(newClusterNodeCommand())

	return cmd
}

func newClusterNodesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "inspect cluster nodes",
		Long: `Inspect cluster nodes.

Queries the server for the set of nodes the cluster that this node knows about.
The output contains the state of each known node.

Examples:
  pico status cluster nodes
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8002",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showClusterNodes(&conf)
	}

	return cmd
}

type clusterNodesOutput struct {
	Nodes []*cluster.Node `json:"nodes"`
}

func showClusterNodes(conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	nodes, err := client.ClusterNodes()
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
	fmt.Println(string(b))
}

func newClusterNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Args:  cobra.ExactArgs(1),
		Short: "inspect cluster node",
		Long: `Inspect a cluster node.

Queries the server for the known state of the node with the given ID. Or use
a node ID of 'local' to query the local node.

Examples:
  # Inspect node bbc69214.
  pico status cluster node bbc69214

  # Inspect local node.
  pico status cluster node local
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8002",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showClusterNode(args[0], &conf)
	}

	return cmd
}

func showClusterNode(nodeID string, conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	node, err := client.ClusterNode(nodeID)
	if err != nil {
		fmt.Printf("failed to get cluster nodes: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(node)
	fmt.Println(string(b))
}
