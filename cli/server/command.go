// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/server"
	"github.com/andydunstall/pico/server/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "start a server node",
		Long: `Start a server node.

The Pico server is responsible for proxying requests from downstream clients to
registered upstream listeners.

Note Pico does not yet support a cluster of nodes.

Examples:
  # Start a pico server on :8080
  pico server

  # Start a pico server on :7000.
  pico server --server.addr :7000
`,
	}

	var conf config.Config

	cmd.Flags().StringVar(&conf.Server.Addr, "server.addr", ":8080", "server listen address")
	cmd.Flags().IntVar(&conf.Server.GracePeriodSeconds, "server.grace-period-seconds", 30, "terminate graceful shutdown period in seconds")

	cmd.Flags().IntVar(&conf.Upstream.HeartbeatIntervalSeconds, "upstream.heartbeat-interval-seconds", 10, "Heartbeat interval in seconds")
	cmd.Flags().IntVar(&conf.Upstream.HeartbeatTimeoutSeconds, "upstream.heartbeat-timeout-seconds", 10, "Heartbeat timeout in seconds")

	cmd.Flags().StringVar(&conf.Log.Level, "log.level", "info", "log level")
	cmd.Flags().StringSliceVar(&conf.Log.Subsystems, "log.subsystems", nil, "enable debug logs for logs the the given subsystems")

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
	server := server.NewServer(
		conf.Server.Addr,
		registry,
		conf,
		logger,
	)

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := server.Serve(); err != nil {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-ctx.Done()

		logger.Info("starting shutdown")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Duration(conf.Server.GracePeriodSeconds)*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
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
