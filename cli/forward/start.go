package forward

import (
	"fmt"
	"os"

	"github.com/andydunstall/piko/forward/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newStartCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags]",
		Short: "open the configured ports",
		Long: `Opens the configured ports for forwards incoming connections
to the configured upstream endpoint.

Examples:
  # Start all ports configured in forward.yaml
  piko forward start --config.file ./forward.yaml
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

		if len(conf.Ports) == 0 {
			fmt.Printf("no ports configured\n")
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
