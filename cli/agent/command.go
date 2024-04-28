package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andydunstall/pico/agent"
	"github.com/andydunstall/pico/agent/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [flags]",
		Short: "start the pico agent",
		Long: `Start the Pico agent.

The Pico agent is a CLI that runs alongside your upstream service that
registers one or more listeners.

The agent will open an outbound connection to a Pico server for each of the
configured listener. This connection is kept open and is used to receive
proxied requets from the server which are then forwarded to the configured
address, then the response is sent back to the Pico server.

Such as if you have a service running at 'localhost:3000', you can register
endpoint 'my-endpoint' that forwards requests to that local service.

Examples:
  # Register a listener with endpoint ID 'my-endpoint-123' that forwards
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

	var conf config.Config

	cmd.Flags().StringSliceVar(
		&conf.Listeners,
		"listeners",
		nil,
		`
The listeners to register with the Pico server.

Each listener registers with an endpoint ID and forward address. The agent
will open a outbound connection to the server for each listener and register
that listener for the endpoint ID. Pico will then forward requests for that
endpoint ID to the agent via the outbound connection, then the agent forwards
the request on to the registered forward address.

'--listeners' is a comma separated list of listeners with format:
'<endpoint ID>/<forward addr>'. Such as '--listeners 6ae6db60/localhost:3000'
will register the listener for endpoint '6ae6db60' then forward incoming
requests to 'localhost:3000'.

You may register multiple listeners which have their own connection to Pico,
such as '--listeners 6ae6db60/localhost:3000,941c3c2e/localhost:4000'.`,
	)

	cmd.Flags().StringVar(
		&conf.Server.URL,
		"server.url",
		"http://localhost:8001",
		`
Pico server URL.

The listener will add path /pico/v1/listener/:endpoint_id to the given URL,
so if you include a path it will be used as a prefix.

Note Pico connects to the server with WebSockets, so will replace http/https
with ws/wss (you can configure either).
`,
	)
	cmd.Flags().IntVar(
		&conf.Server.HeartbeatIntervalSeconds,
		"server.heartbeat-interval-seconds",
		10,
		`
Heartbeat interval in seconds.

To verify the connection to the server is ok, the listener sends a
heartbeat to the upstream at the '--server.heartbeat-interval-seconds'
interval, with a timeout of '--server.heartbeat-timeout-seconds'.`,
	)
	cmd.Flags().IntVar(
		&conf.Server.HeartbeatTimeoutSeconds,
		"server.heartbeat-timeout-seconds",
		10,
		`
Heartbeat timeout in seconds.,

To verify the connection to the server is ok, the listener sends a
heartbeat to the upstream at the '--server.heartbeat-interval-seconds'
interval, with a timeout of '--server.heartbeat-timeout-seconds'.`,
	)

	cmd.Flags().IntVar(
		&conf.Forwarder.TimeoutSeconds,
		"forwarder.timeout",
		10,
		`
Forwarder timeout in seconds.

This is the timeout between a listener receiving a request from Pico then
forwarding it to the configured forward address, and receiving a response.

If the upstream does not respond within the given timeout a
'504 Gateway Timeout' is returned to the client.`,
	)

	cmd.Flags().StringVar(
		&conf.Log.Level,
		"log.level",
		"info",
		`
Minimum log level to output.

The available levels are 'debug', 'info', 'warn' and 'error'.`,
	)
	cmd.Flags().StringSliceVar(
		&conf.Log.Subsystems,
		"log.subsystems",
		nil,
		`
Each log has a 'subsystem' field where the log occured.

'--log.subsystems' enables all log levels for those given subsystems. This
can be useful to debug a particular subsystem without having to enable all
debug logs.

Such as you can enable 'gossip' logs with '--log.subsystems gossip'.`,
	)

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		logger, err := log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		run(&conf, logger)
	}

	return cmd
}

func run(conf *config.Config, logger log.Logger) {
	logger.Info("starting pico agent", zap.Any("conf", conf))

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	for _, l := range conf.Listeners {
		// Already verified format in Config.Validate.
		elems := strings.Split(l, "/")
		endpointID := elems[0]
		forwardAddr := elems[1]

		listener := agent.NewListener(endpointID, forwardAddr, conf, logger)
		g.Go(func() error {
			return listener.Run(ctx)
		})
	}

	if err := g.Wait(); err != nil {
		logger.Error("failed to run agent", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
