package agent

import (
	"fmt"
	"os"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newHTTPCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http [endpoint] [upstream addr] [flags]",
		Short: "register a http listener",
		Long: `Registers a HTTP endpoint with the given endpoint ID and
forwards incoming connections to your upstream service.

The configured upstream address may be a port, a host and port, or a URL.

Examples:
  # Register endpoint 'my-endpoint' for forward incoming connections to
  # localhost:3000.
  piko agent http my-endpoint 3000

  # Register and forward to 10.26.104.56:3000.
  piko agent http my-endpoint 10.26.104.56:3000

  # Register and forward to 10.26.104.56:3000 using HTTPS.
  piko agent http my-endpoint https://10.26.104.56:3000
`,
		Args: cobra.ExactArgs(2),
	}

	var accessLog bool
	cmd.Flags().BoolVar(
		&accessLog,
		"access-log",
		true,
		`
Whether to log all incoming HTTP requests and responses as 'info' logs.`,
	)

	var logger log.Logger

	cmd.PreRun = func(cmd *cobra.Command, args []string) {
		// Discard any endpoints in the configuration file and use from command
		// line.
		conf.Endpoints = nil
		conf.Endpoints = append(conf.Endpoints, config.EndpointConfig{
			ID:        args[0],
			Addr:      args[1],
			AccessLog: accessLog,
		})

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		logger.Info(
			"registered http endpoint",
			zap.String("id", conf.Endpoints[0].ID),
			zap.String("addr", conf.Endpoints[0].Addr),
		)
	}

	return cmd
}
