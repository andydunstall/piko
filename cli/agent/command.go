package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"gopkg.in/yaml.v3"

	"github.com/andydunstall/pico/agent"
	"github.com/andydunstall/pico/agent/config"
	picoconfig "github.com/andydunstall/pico/pkg/config"
	"github.com/andydunstall/pico/pkg/log"
	adminserver "github.com/andydunstall/pico/server/server/admin"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [flags]",
		Short: "start the pico agent",
		Long: `Start the Pico agent.

The Pico agent is a CLI that runs alongside your upstream service that
registers one or more endpoints.

The agent will open an outbound connection to a Pico server for each of the
configured endpoints. This connection is kept open and is used to receive
proxied requests from the server which are then forwarded to the configured
address.

Such as if you have a service running at 'localhost:3000', you can register
endpoint 'my-endpoint' that forwards requests to that local service.

Examples:
  # Register an endpoint with ID 'my-endpoint-123' that forwards requests to
  # to 'localhost:3000'.
  pico agent --endpoints my-endpoint-123/localhost:3000

  # Register multiple endpoints.
  pico agent --endpoints my-endpoint-1/localhost:3000 my-endpoint-2/localhost:6000

  # Specify the Pico server address.
  pico agent --endpoints my-endpoint-123/localhost:3000 \
      --server.url https://pico.example.com
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

		if err := run(&conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func run(conf *config.Config, logger log.Logger) error {
	b, _ := yaml.Marshal(conf)
	fmt.Println(string(b))

	logger.Info("starting pico agent", zap.Any("conf", conf))

	registry := prometheus.NewRegistry()

	adminLn, err := net.Listen("tcp", conf.Admin.BindAddr)
	if err != nil {
		return fmt.Errorf("admin listen: %s: %w", conf.Admin.BindAddr, err)
	}
	adminServer := adminserver.NewServer(
		adminLn,
		registry,
		logger,
	)

	agent := agent.NewAgent(conf, logger)
	agent.Metrics().Register(registry)

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

	// Agent.
	agentCtx, agentCancel := context.WithCancel(context.Background())
	group.Add(func() error {
		if err := agent.Run(agentCtx); err != nil {
			return fmt.Errorf("agent: %w", err)
		}
		return nil
	}, func(error) {
		agentCancel()
	})

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
