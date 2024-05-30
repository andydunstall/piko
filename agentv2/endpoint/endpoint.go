package endpoint

import (
	"context"

	"github.com/andydunstall/piko/agentv2/config"
	piko "github.com/andydunstall/piko/client"
	"github.com/andydunstall/piko/pkg/log"
	"go.uber.org/zap"
)

// Endpoint handles connections for the endpoint and forwards traffic the
// upstream listener.
type Endpoint struct {
	id     string
	logger log.Logger
}

func NewEndpoint(conf config.EndpointConfig, logger log.Logger) *Endpoint {
	return &Endpoint{
		id:     conf.ID,
		logger: logger.WithSubsystem("endpoint").With(zap.String("endpoint-id", conf.ID)),
	}
}

// Serve serves connections on the listener.
func (e *Endpoint) Serve(_ piko.Listener) error {
	e.logger.Info("serving endpoint")

	// TODO(andydunstall): Run reverse proxy to accept connections, log
	// requests and forward to the upstream.

	return nil
}

func (e *Endpoint) Shutdown(_ context.Context) error {
	return nil
}
