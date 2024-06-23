package workload

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "generate test workloads",
		Long: `Generate test workloads.

This tool can be used to run a cluster of Piko server nodes, register
upstreams and generate traffic.

Examples:
  # Start a cluster of 5 nodes.
  piko workload cluster --nodes 5

  # Register 1000 endpoints and upstream servers.
  piko workload upstreams --endpoints 1000

  # Start 10 clients, each sending 5 requests a second where each request is
  # send to a random endpoint.
  piko workload requests --endpoints 1000 --rate 5 --clients 10
`,
	}

	cmd.AddCommand(newClusterCommand())
	cmd.AddCommand(newUpstreamsCommand())
	cmd.AddCommand(newRequestsCommand())

	return cmd
}
