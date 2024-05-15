package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/andydunstall/piko/agent/config"
	"github.com/andydunstall/piko/pkg/log"
	"golang.org/x/sync/errgroup"
)

// Agent is responsible for registering the configured listeners with the Piko
// server for forwarding incoming requests.
type Agent struct {
	conf *config.Config

	metrics *Metrics

	logger log.Logger
}

func NewAgent(conf *config.Config, logger log.Logger) *Agent {
	return &Agent{
		conf:    conf,
		metrics: NewMetrics(),
		logger:  logger.WithSubsystem("agent"),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	g, ctx := errgroup.WithContext(ctx)
	for _, e := range a.conf.Endpoints {
		// Already verified format in Config.Validate.
		elems := strings.Split(e, "/")
		endpointID := elems[0]
		forwardAddr := elems[1]

		endpoint := newEndpoint(endpointID, forwardAddr, a.conf, a.metrics, a.logger)
		g.Go(func() error {
			return endpoint.Run(ctx)
		})
	}

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("endpoint: %s", err)
	}
	return nil
}

func (a *Agent) Metrics() *Metrics {
	return a.metrics
}
