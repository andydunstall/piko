package agent

import "github.com/spf13/cobra"

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [flags]",
		Short: "start the pico agent",
		Long: `Start the Pico agent.

The Pico agent is a CLI that runs alongside your upstream service that
registers one or more listeners.

The agent will connect to a Pico server, register the configured listeners,
then forwards incoming requests to your upstream service.

Such as if you have a service running at 'localhost:3000', you can register
endpoint 'my-endpoint' that forwards requests to that local service.

Examples:
  # Register a listener with endpoint ID 'my-endpoing-123' that forwards
  # requests to 'localhost:3000'.
  pico agent --listener my-endpoint-123/localhost:3000

  # Register multiple listeners.
  pico agent --listener my-endpoint-123/localhost:3000 \
      --listener my-endpoint-xyz/localhost:6000

  # Specify the Pico server address.
  pico agent --listener my-endpoint-123/localhost:3000 \
      --server.url https://pico.example.com
`,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		run()
	}

	return cmd
}

func run() {
}
