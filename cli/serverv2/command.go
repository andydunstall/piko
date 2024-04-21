package serverv2

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/serverv2/config"
	adminserver "github.com/andydunstall/pico/serverv2/server/admin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serverv2",
		Short: "start a server node",
		Long: `Start a server node.

The Pico server is responsible for proxying requests from downstream clients to
registered upstream listeners.

The server has two ports, a 'proxy' port which accepts connections from both
downstream clients and upstream listeners, and an 'admin' port which is used
to inspect the status of the server.

Pico may run as a cluster of nodes for fault tolerance and scalability. Use
'--cluster.members' to configure addresses of existing members in the cluster
to join.

Examples:
  # Start a Pico server.
  pico server

  # Start a Pico server, listening for proxy connections on :7000 and admin
  // ocnnections on :9000.
  pico server --proxy.bind-addr :8000 --admin.bind-addr :9000
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(
		&conf.Proxy.BindAddr,
		"proxy.bind-addr",
		":8080",
		`
The host/port to listen for incoming proxy HTTP and WebSocket connections.

If the host is unspecified it defaults to all listeners, such as
'--proxy.bind-addr :8080' will listen on '0.0.0.0:8080'`,
	)

	cmd.Flags().StringVar(
		&conf.Admin.BindAddr,
		"admin.bind-addr",
		":8081",
		`
The host/port to listen for incoming admin connections.

If the host is unspecified it defaults to all listeners, such as
'--admin.bind-addr :8081' will listen on '0.0.0.0:8081'`,
	)

	cmd.Flags().StringVar(
		&conf.Log.Level,
		"log.level",
		"info",
		`
Minimum log level to output.

The available levels are 'debug', 'info', 'warn' and 'error'.`,
	)
	cmd.Flags().StringSliceVar(
		&conf.Log.Subsystems,
		"log.subsystems",
		nil,
		`
Each log has a 'subsystem' field where the log occured.

'--log.subsystems' enables all log levels for those given subsystems. This
can be useful to debug a particular subsystem without having to enable all
debug logs.

Such as you can enable 'gossip' logs with '--log.subsystems gossip'.`,
	)

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

	registry := prometheus.NewRegistry()

	adminServer := adminserver.NewServer(
		conf.Admin.BindAddr,
		registry,
		logger,
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := adminServer.Serve(); err != nil {
			return fmt.Errorf("admin server serve: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()

		logger.Info("shutting down admin server")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			// TODO(andydunstall): Add configuration.
			time.Second*30,
		)
		defer cancel()

		if err := adminServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown server", zap.Error(err))
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		logger.Error("failed to run server", zap.Error(err))
		os.Exit(1)
	}

	logger.Info("shutdown complete")
}
