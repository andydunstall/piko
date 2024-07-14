package upstreams

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	agentconfig "github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pikotest/workload/upstreams"
	"github.com/andydunstall/piko/pikotest/workload/upstreams/config"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upstreams",
		Short: "add upstream listeners",
		Long: `Add upstream listeners.

Starts the configured number of upstream listeners, which can be either HTTP
or TCP.

HTTP upstreams listen for HTTP requests and echo the request body as a 200
response. TCP upstreams listen for connections and echo incoming bytes.

Endpoint IDs are evenly distributed among the listening upstreams, such as if
you configure 1000 upstreams and 100 endpoints, each endpoint will have 10
upstreams.

Examples:
  # Start 1000 HTTP upstream servers with 100 endpoints.
  piko workload upstreams

  # Start 1000 TCP upstreams.
  piko workload upstreams --protocol tcp

  # Start 5000 HTTP upstream servers with 5000 endpoints (so each upstream has a
  # unique endpoint ID).
  piko workload upstreams --upstreams 5000 --endpoints 5000

  # Configure the Piko server address.
  piko workload upstreams --server.url https://piko.example.com:8001
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
		"starting upstreams",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	nextEndpointID := 0
	for i := 0; i != conf.Upstreams; i++ {
		endpointID := fmt.Sprintf("endpoint-%d", nextEndpointID)
		switch agentconfig.ListenerProtocol(conf.Protocol) {
		case agentconfig.ListenerProtocolHTTP:
			upstream, err := upstreams.NewHTTPUpstream(endpointID, conf, logger)
			if err != nil {
				return fmt.Errorf("upstream: %w", err)
			}
			defer upstream.Close()
		case agentconfig.ListenerProtocolTCP:
			upstream, err := upstreams.NewTCPUpstream(endpointID, conf, logger)
			if err != nil {
				return fmt.Errorf("upstream: %w", err)
			}
			defer upstream.Close()
		default:
			// Already verified so this won't happen.
			panic("unsupported protocol: " + conf.Protocol)
		}

		nextEndpointID++
		nextEndpointID %= conf.Endpoints
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
