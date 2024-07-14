package traffic

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	agentconfig "github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pikotest/workload/traffic"
	"github.com/andydunstall/piko/pikotest/workload/traffic/config"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "traffic",
		Short: "generate downstream traffic",
		Long: `Generates downstream traffic.

Starts the configured number of clients, which connect to upstream listeners,
send random traffic, then verify the request is echoed back.

You can generate either HTTP or TCP traffic. HTTP workloads send HTTP requests
with random bodies and expect the request body to be echoed back. TCP workloads
open connections to upstreams then send random data and expect the data to be
echoed back.

Each request/connection selects a random endpoint ID from the configured number
of endpoints.

Examples:
  # Generate HTTP traffic.
  piko test workload traffic --protocol http

  # Generate TCP traffic.
  piko test workload traffic --protocol tcp

  # Start 10 HTTP clients, each sending 5 requests per second to a random
  # endpoint from 0 to 9999.
  piko test workload traffic --protocol http --endpoints 1000 --rate 5 --clients 10

  # Start 10 TCP clients, each opening a connection and sending a request 5
  # times per second.
  piko test workload traffic --protocol tcp --rate 5 --clients 10
`,
	}

	conf := config.Default()
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.PersistentFlags())
	loadConf.RegisterFlags(cmd.PersistentFlags())

	var logger log.Logger

	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if err := pikoconfig.Load(conf, loadConf.Path, loadConf.ExpandEnv); err != nil {
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
		if err := run(conf, logger); err != nil {
			logger.Error("failed to run upstreams", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func run(conf *config.Config, logger log.Logger) error {
	logger.Info(
		"starting traffic",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	for i := 0; i != conf.Clients; i++ {
		switch agentconfig.ListenerProtocol(conf.Protocol) {
		case agentconfig.ListenerProtocolHTTP:
			client := traffic.NewHTTPClient(conf, logger)
			defer client.Close()
		case agentconfig.ListenerProtocolTCP:
			client := traffic.NewTCPClient(conf, logger)
			defer client.Close()
		default:
			// Already verified so this won't happen.
			panic("unsupported protocol: " + conf.Protocol)
		}
	}

	// Termination handler.
	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-signalCh
	logger.Info(
		"received shutdown signal",
		zap.String("signal", sig.String()),
	)
	return nil
}
