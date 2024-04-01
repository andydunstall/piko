package server

import (
	"fmt"
	"os"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

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

	var conf config.Config

	cmd.Flags().StringVar(&conf.Log.Level, "log.level", "info", "log level")
	cmd.Flags().StringSliceVar(&conf.Log.Subsystems, "log.subsystems", nil, "enable debug logs for logs the the given subsystems")

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

func run(conf *config.Config, logger *log.Logger) {
	logger.Info("starting pico server", zap.Any("conf", conf))
}
