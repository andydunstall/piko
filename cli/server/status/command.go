package status

import (
	"fmt"
	"net/url"
	"os"

	"github.com/andydunstall/piko/server/status/client"
	"github.com/andydunstall/piko/server/status/config"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "inspect server status",
		Long: `Inspect server status.

Each Piko server exposes a status API to inspect the state of the node, this
can be used to answer questions such as:
* What upstream listeners are attached to each node?
* What cluster state does this node know?
* What is the gossip state of each known node?

See 'status --help' for the availale commands.

Examples:
  # Inspect the known nodes in the cluster.
  piko server status cluster nodes
`,
	}

	var conf config.Config
	conf.RegisterFlags(cmd.PersistentFlags())

	c := client.NewClient(nil)

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}

		url, _ := url.Parse(conf.Server.URL)
		c.SetURL(url)
	}

	cmd.AddCommand(newClusterCommand(c))
	cmd.AddCommand(newGossipCommand(c))

	return cmd
}
