package workload

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/andydunstall/piko/pkg/log"
	"github.com/andydunstall/piko/workload/config"
	"github.com/andydunstall/piko/workload/upstream"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

func newUpstreamsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upstreams",
		Short: "add upstream services",
		Long: `Add upstream services.

Starts the configured number of upstream HTTP servers that return status code
200 and echo the request body. Each upstream server has a corresponding agent
that registers an endpoint for that server.

Endpoint IDs will be assigned to upstreams from the number of endpoints. Such
as if you have 1000 upstreams and 100 endpoints, then you'll have 10 upstream
servers per endpoint.

Examples:
  # Start 1000 upstream servers with 100 endpoints.
  piko workload upstreams

  # Start 5000 upstream servers with 5000 endpoints (so each upstream has a
  # unique endpoint ID).
  piko workload upstreams --upstreams 5000 --endpoints 5000

  # Specify the Piko server address.
  piko workload upstreams --server.url https://piko.example.com:8001
`,
	}

	var conf config.UpstreamsConfig

	// Register flags and set default values.
	conf.RegisterFlags(cmd.Flags())

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

		if err := runUpstreams(&conf, logger); err != nil {
			logger.Error("failed to run server", zap.Error(err))
			os.Exit(1)
		}
	}

	return cmd
}

func runUpstreams(conf *config.UpstreamsConfig, logger log.Logger) error {
	logger.Info("starting upstreams workload", zap.Any("conf", conf))

	ctx, cancel := signal.NotifyContext(
		context.Background(), syscall.SIGINT, syscall.SIGTERM,
	)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	nextEndpointID := 0
	for i := 0; i != conf.Upstreams; i++ {
		upstream := upstream.NewUpstream(
			strconv.Itoa(nextEndpointID),
			conf.Server.URL,
			logger,
		)
		g.Go(func() error {
			return runUpstream(ctx, upstream, conf)
		})

		nextEndpointID++
		nextEndpointID %= conf.Endpoints
	}

	return g.Wait()
}

func runUpstream(
	ctx context.Context,
	upstream *upstream.Upstream,
	conf *config.UpstreamsConfig,
) error {
	if conf.Churn.Interval == 0 {
		return upstream.Run(ctx)
	}

	for {
		multipler := rand.Float64()

		churnInterval := time.Duration(float64(conf.Churn.Interval) * multipler)
		upstreamCtx, cancel := context.WithTimeout(ctx, churnInterval)
		defer cancel()

		if err := upstream.Run(upstreamCtx); err != nil {
			if upstreamCtx.Err() == nil {
				return err
			}
		}

		if ctx.Err() != nil {
			return nil
		}

		if conf.Churn.Delay != 0 {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(conf.Churn.Delay):
			}
		}
	}
}
