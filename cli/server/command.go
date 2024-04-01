package server

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a server node",
		Long: `Start a server node.

The Pico server is responsible for proxying requests from downstream clients to
registered upstream listeners.

Note Pico does not yet support a cluster of nodes.

Examples:
  # Start a pico server on :8080
  pico server

  # Start a pico server on :7000.
  pico server --server.addr :7000
`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		run()
	}

	return cmd
}

func run() {
}
