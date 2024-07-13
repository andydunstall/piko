package test

import (
	"github.com/spf13/cobra"

	"github.com/andydunstall/piko/cli/test/workload"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "tools for testing piko clusters",
		Long:  `Tools for testing Piko clusters.`,
	}

	cmd.AddCommand(workload.NewCommand())

	return cmd
}
