package workload

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "generate test workloads",
		Long: `Generate test workloads.

This tool can be used to register upstream endpoints that echo received
requests and generate proxy request traffic.

Examples:
  # Register 1000 endpoints and upstream servers.
  pico workload endpoints --endpoints 1000

  # Start 10 clients, each sending 5 requests a second where each request is
  # send to a random endpoint.
  pico workload requests --endpoints 1000 --rate 5 --clients 10
`,
	}

	cmd.AddCommand(newEndpointsCommand())
	cmd.AddCommand(newRequestsCommand())

	return cmd
}
