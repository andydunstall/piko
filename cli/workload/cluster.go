package workload

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/workload/cluster"
	"github.com/andydunstall/piko/workload/cluster/config"
	"github.com/andydunstall/piko/workload/cluster/proxy"
	rungroup "github.com/oklog/run"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func newClusterCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "create a cluster of Piko nodes",
		Long: `Create a cluster of Piko nodes.

The cluster runs locally and accepts traffic via a load balancer proxy.

Examples:
  # Start a cluster of 5 nodes.
  piko workload cluster --nodes 5
`,
	}

	var conf config.Config
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())
	loadConf.RegisterFlags(cmd.Flags())

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		if err := loadConf.Load(&conf); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runCluster(&conf, logger); err != nil {
			logger.Error("failed to run cluster", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runCluster(conf *config.Config, logger log.Logger) error {
	logger.Info(
		"starting cluster",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	cluster, err := cluster.NewCluster(
		cluster.WithNodes(conf.Nodes),
		cluster.WithLogger(logger),
	)
	if err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if err = cluster.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}
	defer cluster.Stop()

	var group rungroup.Group

	proxyLn, err := net.Listen("tcp", "127.0.0.1:8000")
	if err != nil {
		return fmt.Errorf("proxy listen: %w", err)
	}

	upstreamLn, err := net.Listen("tcp", "127.0.0.1:8001")
	if err != nil {
		return fmt.Errorf("upstream listen: %w", err)
	}

	proxyLb := proxy.NewLoadBalancer(func() []string {
		var addrs []string
		for _, node := range cluster.Nodes() {
			addrs = append(addrs, node.ProxyAddr())
		}
		return addrs
	})
	upstreamLb := proxy.NewLoadBalancer(func() []string {
		var addrs []string
		for _, node := range cluster.Nodes() {
			addrs = append(addrs, node.UpstreamAddr())
		}
		return addrs
	})

	group.Add(func() error {
		if err := proxyLb.Serve(proxyLn); err != nil {
			return fmt.Errorf("proxy lb serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Second*10,
		)
		defer cancel()

		if err := proxyLb.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown proxy lb", zap.Error(err))
		}
	})

	group.Add(func() error {
		if err := upstreamLb.Serve(upstreamLn); err != nil {
			return fmt.Errorf("upstream lb serve: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			time.Second*10,
		)
		defer cancel()

		if err := upstreamLb.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown upstream lb", zap.Error(err))
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

	if err := group.Run(); err != nil {
		return err
	}

	logger.Info("shutdown complete")

	return nil
}
