package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	piko "github.com/andydunstall/piko/agentv2/client"
	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/agentv2/endpoint"
	"github.com/andydunstall/piko/pkg/log"
	rungroup "github.com/oklog/run"
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
		if err := runHTTP(conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runHTTP(conf *config.Config, logger log.Logger) error {
	logger.Info("starting piko agent")
	logger.Warn("piko agent v2 is still in development")

	// We know there is a single endpoint configured.
	endpointConfig := conf.Endpoints[0]
	endpoint := endpoint.NewEndpoint(endpointConfig, logger)

	connTLSConfig, err := conf.Connect.TLS.Load()
	if err != nil {
		return fmt.Errorf("tls: %w", err)
	}

	client := piko.New(
		piko.WithToken(conf.Token),
		piko.WithTLSConfig(connTLSConfig),
		piko.WithLogger(logger.WithSubsystem("client")),
	)

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		conf.Connect.Timeout,
	)
	defer connectCancel()

	ln, err := client.Listen(connectCtx, endpointConfig.ID)
	if err != nil {
		return fmt.Errorf("listen: %s: %w", endpointConfig.ID, err)
	}
	defer ln.Close()

	var group rungroup.Group

	// Endpoint handler.
	group.Add(func() error {
		if err := endpoint.Serve(ln); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			conf.GracePeriod,
		)
		defer cancel()

		if err := endpoint.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown endpoint", zap.Error(err))
		}
	})

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

	return group.Run()
}
