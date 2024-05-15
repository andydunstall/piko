package cli

import (
	"github.com/andydunstall/piko/cli/agent"
	"github.com/andydunstall/piko/cli/server"
	"github.com/andydunstall/piko/cli/status"
	"github.com/andydunstall/piko/cli/workload"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cobra.EnableCommandSorting = false

	cmd := &cobra.Command{
		Use:          "piko [command] (flags)",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Short: "piko proxy",
	}

	cmd.AddCommand(agent.NewCommand())
	cmd.AddCommand(server.NewCommand())
	cmd.AddCommand(status.NewCommand())
	cmd.AddCommand(workload.NewCommand())

	return cmd
}

func init() {
	cobra.EnableCommandSorting = false
}
