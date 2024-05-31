package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/andydunstall/piko/agent"
	"github.com/andydunstall/piko/agent/config"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
	adminserver "github.com/andydunstall/piko/server/server/admin"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [flags]",
		Short: "start the piko agent",
		Long: `Start the Piko agent.

The Piko agent is a command line tool that registers endpoints with Piko and
forwards requests to your upstream service.

To register an endpoint, you configure both the endpoint ID and address to
forward requests to (which will typically be on the same host as the agent).
Such as you may register endpoint 'my-endpoint' that forwards requests to
'localhost:4000'.

For each registered endpoint, the agent will open an outbound-only connection
to Piko. This connection is used to receive proxied requests from the server
which are then forwarded to the configured address.

If multiple upstreams register the same endpoint, Piko load balances requests
for the endpoint among the connected upstreams.

The agent supports both YAML configuration and command line flags. Configure
a YAML file using '--config.path'. When enabling '--config.expand-env', Piko
will expand environment variables in the loaded YAML configuration.

Endpoints can be configured either using the YAML configuration or as command
line arguments. When using command line arguments, each endpoint has format
'<endpoint ID>/<forward addr>', such as 'my-endpoint-123/localhost:3000'.
For more advanced endpoint configurations use the YAML configuration.

Examples:
  # Register an endpoint with ID 'my-endpoint-123' that forwards requests to
  # to 'localhost:3000'.
  piko agent my-endpoint-123/localhost:3000

  # Register multiple endpoints.
  piko agent my-endpoint-1/localhost:3000 my-endpoint-2/localhost:6000

  # Specify the Piko server address.
  piko agent my-endpoint-123/localhost:3000 --server.url https://piko.example.com

  # Load configuration from a YAML file.
  piko agent --config.path ./agent.yaml
`,
	}

	var conf config.Config

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
			if err := pikoconfig.Load(configPath, &conf, configExpandEnv); err != nil {
				fmt.Printf("load config: %s\n", err.Error())
				os.Exit(1)
			}
		}

		for _, arg := range args {
			elems := strings.Split(arg, "/")
			if len(elems) != 2 {
				fmt.Printf("invalid endpoint: %s\n", arg)
				os.Exit(1)
			}

			conf.Endpoints = append(conf.Endpoints, config.EndpointConfig{
				ID:   elems[0],
				Addr: elems[1],
			})
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

		if err := run(&conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func run(conf *config.Config, logger log.Logger) error {
	logger.Info("starting piko agent", zap.Any("conf", conf))

	registry := prometheus.NewRegistry()

	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}
	adminServer := adminserver.NewServer(
		adminLn,
		nil,
		nil,
		registry,
		logger,
	)

	endpointTLSConfig, err := conf.TLS.Load()
	if err != nil {
		return fmt.Errorf("tls: %w", err)
	}

	var group rungroup.Group

	// Termination handler.
	signalCtx, signalCancel := context.WithCancel(context.Background())
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	group.Add(func() error {
		select {
		case sig := <-signalCh:
			logger.Info(
				"received shutdown signal",
				zap.String("signal", sig.String()),
			)
			return nil
		case <-signalCtx.Done():
			return nil
		}
	}, func(error) {
		signalCancel()
	})

	// Endpoints.
	metrics := agent.NewMetrics()
	metrics.Register(registry)

	for _, e := range conf.Endpoints {
		endpoint := agent.NewEndpoint(
			e.ID, e.Addr, conf, endpointTLSConfig, metrics, logger,
		)

		endpointCtx, endpointCancel := context.WithCancel(context.Background())
		group.Add(func() error {
			if err := endpoint.Run(endpointCtx); err != nil {
				return fmt.Errorf("endpoint: %s: %w", e.ID, err)
			}
			return nil
		}, func(error) {
			endpointCancel()
		})
	}

	// Admin server.
	group.Add(func() error {
		if err := adminServer.Serve(); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	}, func(error) {
		if err := adminServer.Close(); err != nil {
			logger.Warn("failed to close server", zap.Error(err))
		}

		logger.Info("admin server shut down")
	})

	if err := group.Run(); err != nil {
		return err
	}

	logger.Info("shutdown complete")

	return nil
}
