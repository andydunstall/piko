package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	upstreamserver "github.com/andydunstall/piko/serverv2/server/upstream"
	rungroup "github.com/oklog/run"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serverv2 [flags]",
		Short: "start a server node",
		Long: `Start a server node.

WARNING: Server V2 is still in development...
`,
		// TODO(andydunstall): Hide while in development.
		Hidden: true,
	}

	cmd.Run = func(cmd *cobra.Command, args []string) {
		logger, err := log.NewLogger("debug", nil)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if err := run(logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func run(logger log.Logger) error {
	logger.Info("starting piko server")
	logger.Warn("piko server v2 is still in development")

	upstreamLn, err := net.Listen("tcp", ":8001")
	if err != nil {
		return fmt.Errorf("upstream listen: %s: %w", ":8001", err)
	}

	upstreamServer := upstreamserver.NewServer(nil, logger)

	var group rungroup.Group

	// Upstream server.
	group.Add(func() error {
		if err := upstreamServer.Serve(upstreamLn); err != nil {
			return fmt.Errorf("upstream server serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Second*10,
		)
		defer cancel()

		if err := upstreamServer.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown upstream server", zap.Error(err))
		}

		logger.Info("upstream server shut down")
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

	if err := group.Run(); err != nil {
		return err
	}

	logger.Info("shutdown complete")

	return nil
}
