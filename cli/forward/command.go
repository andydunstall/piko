package forward

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	rungroup "github.com/oklog/run"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/forward"
	"github.com/andydunstall/piko/forward/config"
	"github.com/andydunstall/piko/pkg/build"
	pikoconfig "github.com/andydunstall/piko/pkg/config"
	"github.com/andydunstall/piko/pkg/log"
)

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward [command] [flags]",
		Short: "forward local ports to an upstream endpoint",
		Long: `Piko forward listens on a local port for forwards to the
configured upstream endpoint.

Such as you may listen on port 3000 and forward connections to endpoint
'my-endpoint'.

Piko forward supports both YAML configuration and command line flags. Configure
a YAML file using '--config.path'. When enabling '--config.expand-env', Piko
will expand environment variables in the loaded YAML configuration.

Examples:
  # Listen for connections on port 3000 and forward to endpoint "my-endpoint".
  piko forward tcp 3000 my-endpoint

  # Start all ports configured in forward.yaml
  piko forward start --config.file ./forward.yaml
`,
	}

	conf := config.Default()
	var loadConf pikoconfig.Config

	// Register flags and set default values.
	conf.RegisterFlags(cmd.PersistentFlags())
	loadConf.RegisterFlags(cmd.PersistentFlags())

	cmd.PersistentPreRun = func(_ *cobra.Command, _ []string) {
		if err := pikoconfig.Load(conf, loadConf.Path, loadConf.ExpandEnv); err != nil {
			fmt.Println(err.Error())
			os.Exit(1)
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("config: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.AddCommand(newStartCommand(conf))
	cmd.AddCommand(newTCPCommand(conf))

	return cmd
}

func runForward(conf *config.Config, logger log.Logger) error {
	logger.Info(
		"starting piko forward",
		zap.String("version", build.Version),
	)
	logger.Debug("piko config", zap.Any("config", conf))

	connectTLSConfig, err := conf.Connect.TLS.Load()
	if err != nil {
		return fmt.Errorf("connect tls: %w", err)
	}

	var group rungroup.Group

	connectURL, err := url.Parse(conf.Connect.URL)
	if err != nil {
		// Already verified in conf.Validate() so this shouldn't happen.
		return fmt.Errorf("connect url: %w", err)
	}
	dialer := &client.Dialer{
		URL:       connectURL,
		Token:     conf.Connect.Token,
		TLSConfig: connectTLSConfig,
	}

	for _, portConfig := range conf.Ports {
		host, _ := portConfig.Host()
		ln, err := net.Listen("tcp", host)
		if err != nil {
			return fmt.Errorf("listen: %s: %w", host, err)
		}

		forwarder := forward.NewForwarder(
			portConfig.EndpointID, dialer, logger.WithSubsystem("forwarder"),
		)

		group.Add(func() error {
			if err := forwarder.Forward(ln); err != nil {
				return fmt.Errorf("serve: %w", err)
			}
			return nil
		}, func(error) {
			if err := forwarder.Close(); err != nil {
				logger.Warn("failed to close forwarder", zap.Error(err))
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
