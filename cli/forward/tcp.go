package forward

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/dragonflydb/piko/forward/config"
	"github.com/dragonflydb/piko/pkg/log"
)

func newTCPCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tcp [addr] [endpoint] [flags]",
		Args:  cobra.ExactArgs(2),
		Short: "open a tcp port",
		Long: `Opens a TCP port and forwards to the configured endpoint.

The configured address may be a port or host and port.

Examples:
  # Listen for connections on port 3000 and forward to endpoint "my-endpoint".
  piko forward tcp 3000 my-endpoint

  # Listen for connections on 0.0.0.0:3000.
  piko forward tcp 0.0.0.0:3000 my-endpoint
`,
	}

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, args []string) {
		// Discard any ports in the configuration file and use from command
		// line.
		conf.Ports = []config.PortConfig{{
			Addr:       args[0],
			EndpointID: args[1],
		}}

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runForward(conf, logger); err != nil {
			logger.Error("failed to run forward", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}
