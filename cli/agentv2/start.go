package agent

import (
	"fmt"
	"os"
	"slices"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newStartCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [endpoint...] [flags]",
		Short: "register the configured endpoints",
		Long: `Registers the configured endpoints with Piko then forwards
incoming connections for each endpoint to your upstream services.

Examples:
  # Start all configured endpoints.
  piko agent start --config.file ./agent.yaml

  # Start only endpoints 'endpoint-1' and 'endpoint-2'.
  piko agent start endpoint-1 endpoint-2 --config.file ./agent.yaml
`,
		Args: cobra.MaximumNArgs(1),
	}

	var logger log.Logger

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if len(conf.Endpoints) == 0 {
			fmt.Printf("no endpoints configured\n")
			os.Exit(1)
		}

		// Verify the requested endpoints to start are configured.
		var endpointIDs []string
		for _, endpoint := range conf.Endpoints {
			endpointIDs = append(endpointIDs, endpoint.ID)
		}
		for _, arg := range args {
			if !slices.Contains(endpointIDs, arg) {
				fmt.Printf("endpoint not found: %s\n", arg)
				os.Exit(1)
			}
		}
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		for _, endpoint := range conf.Endpoints {
			if len(args) != 0 && !slices.Contains(args, endpoint.ID) {
				continue
			}

			logger.Info(
				"registered http endpoint",
				zap.String("id", endpoint.ID),
				zap.String("addr", endpoint.Addr),
			)
		}
	}

	return cmd
}
