package status

import (
	"fmt"
	"os"

	yaml "github.com/goccy/go-yaml"
	"github.com/spf13/cobra"

	"github.com/andydunstall/piko/server/status/client"
)

func newUpstreamCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upstream",
		Short: "inspect connected upstreams",
	}

	cmd.AddCommand(newUpstreamEndpointsCommand(c))

	return cmd
}

func newUpstreamEndpointsCommand(c *client.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "inspect endpoints",
		Long: `Inspect endpoints.

Queries the server for the number of upstream connections for each endpoint.

Examples:
  piko server status upstream endpoints
`,
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		showUpstreamEndpoints(c)
	}

	return cmd
}

func showUpstreamEndpoints(c *client.Client) {
	upstream := client.NewUpstream(c)

	endpoints, err := upstream.Endpoints()
	if err != nil {
		fmt.Printf("failed to get upstream endpoints: %s\n", err.Error())
		os.Exit(1)
	}

	b, _ := yaml.Marshal(endpoints)
	fmt.Print(string(b))
}
