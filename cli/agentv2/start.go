package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/andydunstall/piko/agentv2/config"
	"github.com/andydunstall/piko/agentv2/endpoint"
	piko "github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pkg/log"
	rungroup "github.com/oklog/run"
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
		if err := runStart(args, conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runStart(endpoints []string, conf *config.Config, logger log.Logger) error {
	logger.Info("starting piko agent")
	logger.Warn("piko agent v2 is still in development")

	connectCtx, connectCancel := context.WithTimeout(
		context.Background(),
		conf.Connect.Timeout,
	)
	defer connectCancel()

	client, err := piko.Connect(connectCtx, piko.WithToken(conf.Token))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	var group rungroup.Group

	for _, endpointConfig := range conf.Endpoints {
		if len(endpoints) != 0 && !slices.Contains(endpoints, endpointConfig.ID) {
			continue
		}

		ln, err := client.Listen(context.Background(), endpointConfig.ID)
		if err != nil {
			return fmt.Errorf("listen: %s: %w", endpointConfig.ID, err)
		}

		endpoint := endpoint.NewEndpoint(endpointConfig, logger)

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
	}

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
