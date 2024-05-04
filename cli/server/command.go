package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	picoconfig "github.com/andydunstall/pico/pkg/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server"
	"github.com/andydunstall/pico/server/config"
	rungroup "github.com/oklog/run"
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

Pico may run as a cluster of nodes for fault tolerance and scalability. Use
'--cluster.join' to configure addresses of existing members in the cluster
to join.

Examples:
  # Start a Pico server.
  pico server

  # Start a Pico server, listening for proxy connections on :7000, upstream
  # connections on :7001 and admin connections on :7002.
  pico server --proxy.bind-addr :7000 --upstream.bind-addr :7001 --admin.bind-addr :7002

  # Start a Pico server and join an existing cluster by specifying each member.
  pico server --cluster.join 10.26.104.14,10.26.104.75

  # Start a Pico server and join an existing cluster by specifying a domain.
  # The server will resolve the domain and attempt to join each returned
  # member.
  pico server --cluster.join cluster.pico-ns.svc.cluster.local
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
	server, err := server.NewServer(conf, logger)
	if err != nil {
		return fmt.Errorf("server: %w", err)
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

	runCtx, runCancel := context.WithCancel(context.Background())
	group.Add(func() error {
		return server.Run(runCtx)
	}, func(error) {
		runCancel()
	})

	return group.Run()
}
