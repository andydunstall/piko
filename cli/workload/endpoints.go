package workload

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	picoconfig "github.com/andydunstall/pico/pkg/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/workload/config"
	"github.com/andydunstall/pico/workload/upstream"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func newEndpointsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "endpoints",
		Short: "add upstream endpoints",
		Long: `Add upstream endpoints.

Starts the configured number of upstream HTTP servers that return status code
200 and echo the request body. Each upstream server has a corresponding agent
that registers an endpoint for that server.

Endpoint IDs will be assigned to upstreams from the number of endpoints. Such
as if you have 1000 upstreams and 100 endpoints, then you'll have 10 upstream
servers per endpoint.

Examples:
  # Start 1000 upstream servers with 100 endpoints.
  pico workload endpoints

  # Start 5000 upstream servers with 5000 endpoints (so each upstream has a
  # unique endpoint ID).
  pico workload endpoints --upstreams 5000 --endpoints 5000

  # Specify the Pico server address.
  pico workload endpoints --server.url https://pico.example.com:8001
`,
	}

	var conf config.EndpointsConfig

	var configPath string
	cmd.Flags().StringVar(
		&configPath,
		"config.path",
		"",
		`
YAML config file path.`,
	)

	var configExpandEnv bool
	cmd.Flags().BoolVar(
		&configExpandEnv,
		"config.expand-env",
		false,
		`
Whether to expand environment variables in the config file.

This will replaces references to ${VAR} or $VAR with the corresponding
environment variable. The replacement is case-sensitive.

References to undefined variables will be replaced with an empty string. A
default value can be given using form ${VAR:default}.`,
	)

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if configPath != "" {
			if err := picoconfig.Load(configPath, &conf, configExpandEnv); err != nil {
				fmt.Printf("load config: %s\n", err.Error())
				os.Exit(1)
			}
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		logger, err := log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if err := runEndpoints(&conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runEndpoints(conf *config.EndpointsConfig, logger log.Logger) error {
	logger.Info("starting endpoints workload", zap.Any("conf", conf))

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	nextEndpointID := 0
	for i := 0; i != conf.Upstreams; i++ {
		upstream := upstream.NewUpstream(
			strconv.Itoa(nextEndpointID),
			conf.Server.URL,
			logger,
		)
		g.Go(func() error {
			return upstream.Run(ctx)
		})

		nextEndpointID++
		nextEndpointID %= conf.Endpoints
	}

	return g.Wait()
}
