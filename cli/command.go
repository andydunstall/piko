package cli

import (
	"github.com/andydunstall/pico/cli/agent"
	"github.com/andydunstall/pico/cli/server"
	"github.com/andydunstall/pico/cli/serverv2"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	cobra.EnableCommandSorting = false

	cmd := &cobra.Command{
		Use:          "pico [command] (flags)",
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		Short: "pico proxy",
	}

	cmd.AddCommand(agent.NewCommand())
	cmd.AddCommand(server.NewCommand())
	cmd.AddCommand(serverv2.NewCommand())

	return cmd
}

func init() {
	cobra.EnableCommandSorting = false
}
