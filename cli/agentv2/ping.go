package agent

import (
	"github.com/andydunstall/piko/agentv2/config"
	"github.com/spf13/cobra"
)

func newPingCommand(_ *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ping [flags]",
		Short: "ping the piko server",
		Long: `Connects to and authenticates with the Piko server, then
measures the round trip latency.
`,
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
	}

	return cmd
}
