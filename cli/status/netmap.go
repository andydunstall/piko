package status

import (
	"fmt"
	"net/url"
	"os"
	"sort"

	"github.com/andydunstall/pico/server/netmap"
	"github.com/andydunstall/pico/status/client"
	"github.com/andydunstall/pico/status/config"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newNetmapCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "netmap",
		Short: "inspect proxy netmap",
	}

	cmd.AddCommand(newNetmapNodesCommand())
	cmd.AddCommand(newNetmapNodeCommand())

	return cmd
}

func newNetmapNodesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "inspect netmap nodes",
		Long: `Inspect netmap nodes.

Queries the server for the set of nodes the cluster that this node knows about.
The output contains the state of each known node.

Examples:
  pico status netmap nodes
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8081",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showNetmapNodes(&conf)
	}

	return cmd
}

type netmapNodesOutput struct {
	Nodes []*netmap.Node `json:"nodes"`
}

func showNetmapNodes(conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	nodes, err := client.NetmapNodes()
	if err != nil {
		fmt.Printf("failed to get netmap nodes: %s\n", err.Error())
		os.Exit(1)
	}

	// Sort by ID.
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	output := netmapNodesOutput{
		Nodes: nodes,
	}
	b, _ := yaml.Marshal(output)
	fmt.Println(string(b))
}

func newNetmapNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Args:  cobra.ExactArgs(1),
		Short: "inspect netmap node",
		Long: `Inspect a netmap node.

Queries the server for the known state of the node with the given ID. Or use
a node ID of 'local' to query the local node.

Examples:
  # Inspect node bbc69214.
  pico status netmap node bbc69214

  # Inspect local node.
  pico status netmap node local
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8081",
		`
Pico server URL. This URL should point to the server admin port.
`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		showNetmapNode(args[0], &conf)
	}

	return cmd
}

func showNetmapNode(nodeID string, conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	node, err := client.NetmapNode(nodeID)
	if err != nil {
		fmt.Printf("failed to get netmap nodes: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(node)
	fmt.Println(string(b))
}
