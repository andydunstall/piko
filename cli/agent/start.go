package agent

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
)

func newStartCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags]",
		Short: "register the configured listeners",
		Long: `Registers the configured listeners with Piko and forwards
incoming connections for each listener to your upstream services.

Examples:
  # Start all listeners configured in agent.yaml.
  piko agent start --config.file ./agent.yaml
`,
	}

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if len(conf.Listeners) == 0 {
			fmt.Printf("no listeners configured\n")
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
