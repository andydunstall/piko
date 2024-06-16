package agent

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/andydunstall/piko/agent/client"
	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/agent/reverseproxy"
	"github.com/andydunstall/piko/agent/server"
	"github.com/andydunstall/piko/agent/tcpproxy"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
	rungroup "github.com/oklog/run"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "agent [command] [flags]",
		Short: "register endpoints and forward requests to your upstream services",
		Long: `The Piko agent registers endpoints with Piko, then listens
for connections on those endpoints and forwards them to your upstream services.

Such as you may listen on endpoint 'my-endpoint' and forward connections to
your service at 'localhost:3000'.

The agent opens an outbound connection to the Piko server for each listener,
then incoming connections from Piko are multiplexed over that outbound
connection. Therefore the agent never exposes a port.

If there are multiple listeners for the same endpoint, Piko load balances
requests the registered listeners.

Piko supports HTTP and TCP listeners. HTTP listeners parse and log each request
before forwarding it to the upstream, whereas TCP listeners forward raw
connections.

The agent supports both YAML configuration and command line flags. Configure
a YAML file using '--config.path'. When enabling '--config.expand-env', Piko
will expand environment variables in the loaded YAML configuration.

Examples:
  # Listen for HTTP requests from endpoint 'my-endpoint' and forward to
  # localhost:3000.
  piko agent http my-endpoint 3000

  # Listen for TCP connections from endpoint 'my-endpoint' and forward to
  # localhost:3000.
  piko agent tcp my-endpoint 3000

  # Start all listeners configured in agent.yaml.
  piko agent start --config.file ./agent.yaml
`,
	}

	var conf config.Config
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.PersistentFlags())
	loadConf.RegisterFlags(cmd.PersistentFlags())

	cmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if err := loadConf.Load(&conf); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		// Listener protocol defaults to HTTP.
		for _, listener := range conf.Listeners {
			if listener.Protocol == "" {
				listener.Protocol = config.ListenerProtocolHTTP
			}
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.AddCommand(newStartCommand(&conf))
	cmd.AddCommand(newHTTPCommand(&conf))
	cmd.AddCommand(newTCPCommand(&conf))

	return cmd
}

func runAgent(conf *config.Config, logger log.Logger) error {
	logger.Info(
		"starting piko agent",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	connectTLSConfig, err := conf.Connect.TLS.Load()
	if err != nil {
		return fmt.Errorf("connect tls: %w", err)
	}

	client := client.New(
		client.WithToken(conf.Connect.Token),
		client.WithUpstreamURL(conf.Connect.URL),
		client.WithTLSConfig(connectTLSConfig),
		client.WithLogger(logger.WithSubsystem("client")),
	)

	registry := prometheus.NewRegistry()

	var group rungroup.Group

	for _, listenerConfig := range conf.Listeners {
		connectCtx, connectCancel := context.WithTimeout(
			context.Background(),
			conf.Connect.Timeout,
		)
		defer connectCancel()

		ln, err := client.Listen(connectCtx, listenerConfig.EndpointID)
		if err != nil {
			return fmt.Errorf("listen: %s: %w", listenerConfig.EndpointID, err)
		}
		defer ln.Close()

		if listenerConfig.Protocol == config.ListenerProtocolHTTP {
			server := reverseproxy.NewServer(listenerConfig, registry, logger)

			// Listener handler.
			group.Add(func() error {
				if err := server.Serve(ln); err != nil {
					return fmt.Errorf("serve: %w", err)
				}
				return nil
			}, func(error) {
				shutdownCtx, cancel := context.WithTimeout(
					context.Background(), conf.GracePeriod,
				)
				defer cancel()

				if err := server.Shutdown(shutdownCtx); err != nil {
					logger.Warn("failed to gracefully shutdown listener", zap.Error(err))
				}
			})
		} else if listenerConfig.Protocol == config.ListenerProtocolTCP {
			server := tcpproxy.NewServer(listenerConfig, logger)

			// Listener handler.
			group.Add(func() error {
				if err := server.Serve(ln); err != nil {
					return fmt.Errorf("serve: %w", err)
				}
				return nil
			}, func(error) {
				if err := server.Close(); err != nil {
					logger.Warn("failed to close listener", zap.Error(err))
				}
			})
		}
	}

	// Agent server.
	serverLn, err := net.Listen("tcp", conf.Server.BindAddr)
	if err != nil {
		return fmt.Errorf("server listen: %s: %w", conf.Server.BindAddr, err)
	}
	server := server.NewServer(registry, logger)

	group.Add(func() error {
		if err := server.Serve(serverLn); err != nil {
			return fmt.Errorf("agent server: %w", err)
		}
		return nil
	}, func(error) {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(), conf.GracePeriod,
		)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			logger.Warn("failed to gracefully shutdown agent server", zap.Error(err))
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
