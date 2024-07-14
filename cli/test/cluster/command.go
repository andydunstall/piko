package cluster

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	rungroup "github.com/oklog/run"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/pikotest/cluster"
	"github.com/andydunstall/piko/pikotest/cluster/config"
	"github.com/andydunstall/piko/pikotest/cluster/proxy"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "create a cluster of Piko nodes",
		Long: `Create a cluster of Piko nodes.

The cluster runs the configured number of nodes with a load balancer proxy.

To test nodes joining and leaving the cluster, configure the 'churn' interval
which defines how often to stop a node and replace it with a new one.

Supports both YAML configuration and command line flags. Configure a YAML file
using '--config.path'. When enabling '--config.expand-env', Piko will expand
environment variables in the loaded YAML configuration.

The cluster configuration is dynamic and can be reloaded by sending a SIGHUP
signal to the process.

Examples:
  # Start a cluster of 5 nodes.
  piko workload cluster --nodes 5

  # Start a cluster and replace one node every 10 seconds.
  piko workload cluster --churn.interval 10s
`,
	}

	conf := config.Default()
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())
	loadConf.RegisterFlags(cmd.Flags())

	var logger log.Logger

	loadConfig := func() error {
		if err := pikoconfig.Load(conf, loadConf.Path, loadConf.ExpandEnv); err != nil {
			return fmt.Errorf("load: %w", err)
		}

		if err := conf.Validate(); err != nil {
			return fmt.Errorf("validate: %w", err)
		}

		return nil
	}

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		if err := loadConfig(); err != nil {
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
		if err := runCluster(conf, loadConfig, logger); err != nil {
			logger.Error("failed to run cluster", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runCluster(
	conf *config.Config,
	loadConfig func() error,
	logger log.Logger,
) error {
	logger.Info(
		"starting cluster",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	defer func() {
		logger.Info("shutdown complete")
	}()

	manager := cluster.NewManager(cluster.WithLogger(logger))
	defer manager.Close()

	manager.Update(conf)

	loadBalancer := proxy.NewLoadBalancer(manager)
	defer loadBalancer.Close()

	var group rungroup.Group

	// Config reload.
	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	hupCancel := make(chan struct{})
	group.Add(func() error {
		for {
			select {
			case <-hup:
				logger.Info("received hup signal")

				if err := loadConfig(); err != nil {
					logger.Error("failed to load config", zap.Error(err))
					continue
				}

				manager.Update(conf)
			case <-hupCancel:
				return nil
			}
		}
	}, func(error) {
		close(hupCancel)
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

	return nil
}
