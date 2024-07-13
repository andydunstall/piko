package workload

import (
	"github.com/spf13/cobra"

	"github.com/andydunstall/piko/cli/test/workload/traffic"
	"github.com/andydunstall/piko/cli/test/workload/upstreams"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload",
		Short: "tools for generating test piko workloads",
		Long: `Tools for generating test Piko workloads.

This includes adding upstream listeners and generating HTTP and TCP traffic.

Examples:
  # Register HTTP upstreams.
  piko test workload upstreams --protocol http

  # Register TCP upstreams.
  piko test workload upstreams --protocol tcp

  # Generate HTTP traffic.
  piko test workload traffic --protocol http

  # Generate TCP traffic.
  piko test workload traffic --protocol tcp
`,
	}

	cmd.AddCommand(upstreams.NewCommand())
	cmd.AddCommand(traffic.NewCommand())

	return cmd
}
