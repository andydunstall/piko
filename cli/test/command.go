package test

import (
	"github.com/spf13/cobra"

	"github.com/andydunstall/piko/cli/test/cluster"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test",
		Short: "tools for testing piko clusters",
		Long:  `Tools for testing Piko clusters.`,
	}

	cmd.AddCommand(cluster.NewCommand())

	return cmd
}
