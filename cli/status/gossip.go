package status

import (
	"fmt"
	"net/url"
	"os"
	"sort"

	"github.com/andydunstall/kite"
	"github.com/andydunstall/pico/status/client"
	"github.com/andydunstall/pico/status/config"
	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"
)

func newGossipCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gossip",
		Short: "inspect gossip state",
	}

	cmd.AddCommand(newGossipNodesCommand())
	cmd.AddCommand(newGossipNodeCommand())

	return cmd
}

func newGossipNodesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "nodes",
		Short: "inspect gossip nodes",
		Long: `Inspect gossip nodes.

Queries the server for the metadata for each known gossip node in the
cluster.

Examples:
  pico status gossip nodes
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

		showGossipNodes(&conf)
	}

	return cmd
}

type gossipNodesOutput struct {
	Nodes []kite.NodeMetadata `json:"nodes"`
}

func showGossipNodes(conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	nodes, err := client.GossipNodes()
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

func newGossipNodeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Args:  cobra.ExactArgs(1),
		Short: "inspect a gossip node",
		Long: `Inspect a gossip node.

Queries the server for the known state of the gossip node with the given ID.

Examples:
  pico status gossip node bbc69214
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

		showGossipNode(args[0], &conf)
	}

	return cmd
}

func showGossipNode(nodeID string, conf *config.Config) {
	// The URL has already been validated in conf.
	url, _ := url.Parse(conf.Server.URL)
	client := client.NewClient(url)
	defer client.Close()

	node, err := client.GossipNode(nodeID)
	if err != nil {
		fmt.Printf("failed to get gossip node: %s: %s\n", nodeID, err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(node)
	fmt.Println(string(b))
}
