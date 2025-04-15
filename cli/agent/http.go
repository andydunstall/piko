package agent

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
)

func newHTTPCommand(conf *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "http [endpoint] [addr] [flags]",
		Args:  cobra.ExactArgs(2),
		Short: "register a http listener",
		Long: `Listens for HTTP traffic on the given endpoint and forwards
incoming connections to your upstream service.

The configured upstream address be a port, host and port or a full URL.

Examples:
  # Listen for connections from endpoint 'my-endpoint' and forward connections
  # to localhost:3000.
  piko agent http my-endpoint 3000

  # Listen and forward to 10.26.104.56:3000.
  piko agent http my-endpoint 10.26.104.56:3000

  # Listen and forward to 10.26.104.56:3000 using HTTPS.
  piko agent http my-endpoint https://10.26.104.56:3000
`,
	}

	accessLogConfig := log.AccessLogConfig{
		Level:   "info",
		Disable: false,
	}
	flags := cmd.Flags()
	accessLogConfig.RegisterFlags(flags, "")

	var timeout time.Duration
	flags.DurationVar(
		&timeout,
		"timeout",
		time.Second*10,
		`
Timeout forwarding incoming HTTP requests to the upstream.`,
	)

	var httpClientConfig config.ListenerHTTPClientConfig
	flags.DurationVar(
		&httpClientConfig.KeepAliveTimeout,
		"http-client.keep-alive-timeout",
		30*time.Second,
		`
 HTTP dialer keep-alive timeout in seconds.`,
	)
	flags.DurationVar(
		&httpClientConfig.IdleConnTimeout,
		"http-client.idle-conn-timeout",
		90*time.Second,
		`
 HTTP transport idle connection timeout in seconds.`,
	)
	flags.IntVar(
		&httpClientConfig.MaxIdleConns,
		"http-client.max-idle-conns",
		100,
		`
 HTTP transport maximum number of idle connections allowed.`,
	)
	flags.BoolVar(
		&httpClientConfig.DisableCompression,
		"http-client.disable-compression",
		false,
		`
 HTTP transport disable accepting compressed responses.`,
	)

	var logger log.Logger

	cmd.PreRun = func(_ *cobra.Command, args []string) {
		// Discard any listeners in the configuration file and use from command
		// line.
		conf.Listeners = []config.ListenerConfig{{
			EndpointID: args[0],
			Addr:       args[1],
			Protocol:   config.ListenerProtocolHTTP,
			AccessLog:  accessLogConfig,
			Timeout:    timeout,
			HTTPClient: httpClientConfig,
		}}

		var err error
		logger, err = log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}
	}

	cmd.Run = func(_ *cobra.Command, _ []string) {
		if err := runAgent(conf, logger); err != nil {
			logger.Error("failed to run agent", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}
