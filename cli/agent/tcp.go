package agent

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
)

func newTCPCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tcp [endpoint] [addr] [flags]",
		Args:  cobra.ExactArgs(2),
		Short: "register a tcp listener",
		Long: `Listens for TCP traffic on the given endpoint and forwards
incoming connections to your upstream service.

The configured upstream address be a port or host and port.

Examples:
  # Listen for connections from endpoint 'my-endpoint' and forward
  # to localhost:3000.
  piko agent tcp my-endpoint 3000

  # Listen and forward to 10.26.104.56:3000.
  piko agent tcp my-endpoint 10.26.104.56:3000
`,
	}

	var accessLog bool
	cmd.Flags().BoolVar(
		&accessLog,
		"access-log",
		true,
		`
Whether to log all incoming connections as 'info' logs.`,
	)

	var timeout time.Duration
	cmd.Flags().DurationVar(
		&timeout,
		"timeout",
		time.Second*10,
		`
Timeout connecting to the upstream.`,
	)

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, args []string) {
		// Discard any listeners in the configuration file and use from command
		// line.
		conf.Listeners = []config.ListenerConfig{{
			EndpointID: args[0],
			Addr:       args[1],
			Protocol:   config.ListenerProtocolTCP,
			AccessLog:  accessLog,
			Timeout:    timeout,
		}}

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runAgent(conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}
