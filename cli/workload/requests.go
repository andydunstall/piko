package workload

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	picoconfig "github.com/andydunstall/pico/pkg/config"
	"github.com/andydunstall/pico/pkg/log"
	"github.com/andydunstall/pico/workload/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func newRequestsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "requests",
		Short: "generate proxy request traffic",
		Long: `Generate proxy request traffic.

Starts the configured number of clients sending HTTP requests to Pico which
are proxied to the upstream endpoints.

Each request selects a random endpoint ID from the configured number of
endpoints.

By default requests are empty, though you can configure the payload size of
each message.

Examples:
  # Run 50 clients with 10 requests per second using 100 endpoints.
  pico workload requests

  # Run 100 clients with 2 requests per second using 50 endpoints.
  pico workload requests --clients 100 --rate 2 --endpoints 50

  # Specify the Pico server address.
  pico workload requests --server.url https://pico.example.com:8000

  # Specify the request payload size.
  pico workload requests --request.size 1024
`,
	}

	var conf config.RequestsConfig

	var configPath string
	cmd.Flags().StringVar(
		&configPath,
		"config.path",
		"",
		`
YAML config file path.`,
	)

	var configExpandEnv bool
	cmd.Flags().BoolVar(
		&configExpandEnv,
		"config.expand-env",
		false,
		`
Whether to expand environment variables in the config file.

This will replaces references to ${VAR} or $VAR with the corresponding
environment variable. The replacement is case-sensitive.

References to undefined variables will be replaced with an empty string. A
default value can be given using form ${VAR:default}.`,
	)

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())

	cmd.Run = func(cmd *cobra.Command, args []string) {
		if configPath != "" {
			if err := picoconfig.Load(configPath, &conf, configExpandEnv); err != nil {
				fmt.Printf("load config: %s\n", err.Error())
				os.Exit(1)
			}
		}

		if err := conf.Validate(); err != nil {
			fmt.Printf("invalid config: %s\n", err.Error())
			os.Exit(1)
		}

		logger, err := log.NewLogger(conf.Log.Level, conf.Log.Subsystems)
		if err != nil {
			fmt.Printf("failed to setup logger: %s\n", err.Error())
			os.Exit(1)
		}

		if err := runRequests(&conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runRequests(conf *config.RequestsConfig, logger log.Logger) error {
	logger.Info("starting requests workload", zap.Any("conf", conf))

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for i := 0; i != conf.Clients; i++ {
		g.Go(func() error {
			return runClient(ctx, conf, logger)
		})
	}

	return g.Wait()
}

func runClient(ctx context.Context, conf *config.RequestsConfig, logger log.Logger) error {
	ticker := time.NewTicker(time.Duration(int(time.Second) / conf.Rate))
	defer ticker.Stop()

	client := &http.Client{}
	for {
		select {
		case <-ticker.C:
			endpointID := rand.Int() % conf.Endpoints
			req, _ := http.NewRequest("GET", conf.Server.URL, nil)
			req.Header.Set("x-pico-endpoint", strconv.Itoa(endpointID))
			resp, err := client.Do(req)
			if err != nil {
				logger.Warn("request", zap.Error(err))
				continue
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logger.Warn("bad status", zap.Int("status", resp.StatusCode))
			}
		case <-ctx.Done():
			return nil
		}
	}
}
