package bench

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/bench"
	"github.com/andydunstall/piko/bench/config"
	"github.com/andydunstall/piko/pkg/build"
	"github.com/andydunstall/piko/pkg/log"
)

func newHTTPCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http [flags]",
		Short: "http request benchmark",
		Long: `Benchmark Piko HTTP.

Benchmarks Piko by sending HTTP requests.

Examples:
  # Benchmark 100000 HTTP requests from 50 clients.
  piko bench http --requests 100000 --clients 50

  # Benchmark with 500 upstreams and 100 endpoints.
  piko bench http --upstreams 500 --endpoints 100

  # Benchmarks with requests size 4096 bytes.
  piko bench http --size 4096
`,
	}

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, _ []string) {
		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runHTTP(conf, logger); err != nil {
			logger.Error("failed to run benchmark", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runHTTP(conf *config.Config, logger log.Logger) error {
	logger.Info(
		"starting http benchmark",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	b := bench.NewHTTPBenchmark(conf, logger)
	return b.Run(ctx)
}
