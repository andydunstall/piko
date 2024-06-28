package workload

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workloadv2",
		Short: "generate test workloads",
		Long: `Generate test workloads.

Piko workload can run a cluster of Piko server nodes, inject faults, register
upstreams and generate traffic from proxy clients. It is used to stress Piko
under different scenarios.

Examples:
  # Start a cluster of 5 nodes.
  piko workload cluster --nodes 5
`,
	}

	cmd.AddCommand(newClusterCommand())

	return cmd
}
