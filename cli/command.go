package cli

import (
	"github.com/andydunstall/piko/cli/agent"
	"github.com/andydunstall/piko/cli/forward"
	"github.com/andydunstall/piko/cli/server"
	"github.com/andydunstall/piko/cli/workload"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "piko [command] (flags)",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Long: `Piko is a reverse proxy that allows you to expose an endpoint
that isn’t publicly routable (known as tunnelling).

The Piko server is responsible for routing incoming proxy requests to upstream
services. Upstream services open outbound-connections to the server and
register endpoints. Piko will then route incoming requests to the appropriate
upstream service via the upstreams outbound-only connection.

The server may be hosted as a cluster of nodes.

Start a server node with:

  $ piko server

You can also inspect the status of the server using:

  $ piko server status

To register an upstream service, use the Piko agent. The agent is a lightweight
proxy that runs alongside your services. It connects to the Piko server,
registers the configured endpoints, then forwards incoming requests to your
services.

Such as to register endpoint 'my-endpoint' to forward HTTP requests to your
service at 'localhost:3000':

  $ piko agent http my-endpoint 3000

You can also forward raw TCP using:

  $ piko agent tcp my-endpoint 3000

To forward a local TCP port to an upstream endpoint, use 'piko forward'.
This listens for TCP connections on the configured local port and forwards them
to an upstream listener via Piko. Such as to forward port 3000 to endpoint
'my-endpoint':

  $ piko forward tcp 3000 my-endpoint

`,
	}

	cmd.AddCommand(server.NewCommand())
	cmd.AddCommand(agent.NewCommand())
	cmd.AddCommand(forward.NewCommand())
	cmd.AddCommand(workload.NewCommand())

	return cmd
}

func init() {
	cobra.EnableCommandSorting = false
}
